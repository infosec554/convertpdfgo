// Package accesslog records every backend request (tool/API actions) into an
// access_log table so the admin console can show real traffic: who (IP),
// what action (method + path), when, which account (or guest), and — once a
// geo source is wired — the country. It is intentionally self-contained: its
// own small pool, its own CREATE TABLE IF NOT EXISTS, and best-effort async
// inserts that never block or fail a request.
package accesslog

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"convertpdfgo/config"
	"convertpdfgo/pkg/logger"
)

var (
	pool *pgxpool.Pool
	lg   logger.ILogger
)

// skip paths that are pure noise (health probes, metrics, the favicon, etc.).
func skip(method, path string) bool {
	if method == "OPTIONS" {
		return true
	}
	switch path {
	case "/healthz", "/readyz", "/metrics", "/favicon.ico":
		return true
	}
	return strings.HasPrefix(path, "/swagger") || strings.HasPrefix(path, "/ws")
}

// Init builds the pool and ensures the table. Failure is logged, not fatal —
// the middleware simply no-ops when the pool is nil.
func Init(ctx context.Context, cfg config.Config, log logger.ILogger) {
	lg = log
	url := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		cfg.PostgresUser, cfg.PostgresPassword, cfg.PostgresHost, cfg.PostgresPort, cfg.PostgresDB)

	pc, err := pgxpool.ParseConfig(url)
	if err != nil {
		log.Error("accesslog: parse config", logger.Error(err))
		return
	}
	pc.MaxConns = 3
	pc.MinConns = 0
	pc.MaxConnIdleTime = 5 * time.Minute

	p, err := pgxpool.NewWithConfig(ctx, pc)
	if err != nil {
		log.Error("accesslog: pool", logger.Error(err))
		return
	}

	const ddl = `
		CREATE TABLE IF NOT EXISTS access_log (
			id          bigserial PRIMARY KEY,
			created_at  timestamptz NOT NULL DEFAULT now(),
			ip          text NOT NULL DEFAULT '',
			country     text NOT NULL DEFAULT '',
			method      text NOT NULL DEFAULT '',
			path        text NOT NULL DEFAULT '',
			status      int  NOT NULL DEFAULT 0,
			user_id     uuid,
			user_agent  text NOT NULL DEFAULT '',
			referer     text NOT NULL DEFAULT ''
		);
		CREATE INDEX IF NOT EXISTS access_log_created_idx ON access_log (created_at DESC);
		CREATE INDEX IF NOT EXISTS access_log_country_idx ON access_log (country);
	`
	if _, err := p.Exec(ctx, ddl); err != nil {
		log.Error("accesslog: ensure table", logger.Error(err))
		p.Close()
		return
	}
	pool = p
	log.Info("accesslog: ready")
}

// Middleware records each request after it completes.
func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if pool == nil {
			return
		}
		method, path := c.Request.Method, c.Request.URL.Path
		if skip(method, path) {
			return
		}

		// Real client IP (gin reads X-Forwarded-For); country comes from
		// Cloudflare's CF-IPCountry header when proxied, else blank (a geoip
		// lookup can fill it later).
		ip := c.ClientIP()
		country := c.GetHeader("CF-IPCountry")
		if country == "XX" {
			country = ""
		}
		var userID *string
		if uid := c.GetString("user_id"); uid != "" {
			userID = &uid
		}
		ua := c.Request.UserAgent()
		ref := c.Request.Referer()
		status := c.Writer.Status()

		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_, err := pool.Exec(ctx,
				`INSERT INTO access_log (ip, country, method, path, status, user_id, user_agent, referer)
				 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
				ip, country, method, path, status, userID, ua, ref)
			if err != nil && lg != nil {
				lg.Warning("accesslog: insert", logger.Error(err))
			}
		}()
	}
}

// Entry is one recorded request.
type Entry struct {
	CreatedAt time.Time `json:"created_at"`
	IP        string    `json:"ip"`
	Country   string    `json:"country"`
	Method    string    `json:"method"`
	Path      string    `json:"path"`
	Status    int       `json:"status"`
	UserID    *string   `json:"user_id,omitempty"`
	UserAgent string    `json:"user_agent"`
	Referer   string    `json:"referer"`
}

// Recent returns the latest requests, newest first.
func Recent(ctx context.Context, limit int) ([]Entry, error) {
	if pool == nil {
		return nil, nil
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := pool.Query(ctx,
		`SELECT created_at, ip, country, method, path, status, user_id, user_agent, referer
		 FROM access_log ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Entry
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.CreatedAt, &e.IP, &e.Country, &e.Method, &e.Path, &e.Status, &e.UserID, &e.UserAgent, &e.Referer); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// Bucket is a label + count, used for country / top-path aggregates.
type Bucket struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

// Summary returns total requests, unique IPs, and top countries/paths over
// the last `hours` window.
func Summary(ctx context.Context, hours int) (total, uniqueIPs int, countries, paths []Bucket, err error) {
	if pool == nil {
		return 0, 0, nil, nil, nil
	}
	if hours <= 0 || hours > 24*90 {
		hours = 24
	}
	since := fmt.Sprintf("%d hours", hours)

	_ = pool.QueryRow(ctx,
		`SELECT count(*), count(DISTINCT ip) FROM access_log WHERE created_at >= now() - $1::interval`, since).
		Scan(&total, &uniqueIPs)

	collect := func(q string) ([]Bucket, error) {
		rows, e := pool.Query(ctx, q, since)
		if e != nil {
			return nil, e
		}
		defer rows.Close()
		var bs []Bucket
		for rows.Next() {
			var b Bucket
			if e := rows.Scan(&b.Label, &b.Count); e != nil {
				return nil, e
			}
			bs = append(bs, b)
		}
		return bs, rows.Err()
	}

	countries, _ = collect(`SELECT COALESCE(NULLIF(country,''),'—'), count(*) FROM access_log
		WHERE created_at >= now() - $1::interval GROUP BY 1 ORDER BY 2 DESC LIMIT 12`)
	paths, _ = collect(`SELECT path, count(*) FROM access_log
		WHERE created_at >= now() - $1::interval GROUP BY 1 ORDER BY 2 DESC LIMIT 12`)
	return total, uniqueIPs, countries, paths, nil
}
