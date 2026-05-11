package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/storage"
)

type adminStatsRepo struct {
	db  *pgxpool.Pool
	log logger.ILogger
}

func NewAdminStatsRepo(db *pgxpool.Pool, log logger.ILogger) storage.IAdminStatsStorage {
	return &adminStatsRepo{db: db, log: log}
}

// UsersByDay — generate_series guarantees a row for empty days too,
// so the resulting chart has no gaps.
func (r *adminStatsRepo) UsersByDay(ctx context.Context, days int) ([]models.DailyCount, error) {
	if days <= 0 || days > 365 {
		days = 30
	}
	const q = `
		WITH days AS (
			SELECT generate_series(
				current_date - ($1::int - 1),
				current_date,
				'1 day'::interval
			)::date AS day
		)
		SELECT to_char(d.day, 'YYYY-MM-DD'),
		       COALESCE(c.cnt, 0) AS cnt
		FROM days d
		LEFT JOIN (
			SELECT created_at::date AS day, COUNT(*) AS cnt
			FROM users
			WHERE created_at >= current_date - ($1::int - 1)
			GROUP BY 1
		) c ON c.day = d.day
		ORDER BY d.day
	`
	rows, err := r.db.Query(ctx, q, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]models.DailyCount, 0, days)
	for rows.Next() {
		var dc models.DailyCount
		if err := rows.Scan(&dc.Day, &dc.Count); err != nil {
			r.log.Error("admin_stats: scan day", logger.Error(err))
			continue
		}
		out = append(out, dc)
	}
	return out, nil
}

// UserBuckets aggregates the five summary counters in a single round-trip.
// Replaces the previous handler that issued FIVE User.List() calls (10 SQL
// queries — each List does a SELECT + COUNT(*)). One scan, FILTER does the
// rest.
func (r *adminStatsRepo) UserBuckets(ctx context.Context) (map[string]int, error) {
	const q = `
		SELECT
			COUNT(*)                                                AS total,
			COUNT(*) FILTER (WHERE status = 'active')               AS active,
			COUNT(*) FILTER (WHERE status = 'blocked')              AS blocked,
			COUNT(*) FILTER (WHERE role   = 'admin')                AS admins,
			COUNT(*) FILTER (WHERE role   = 'moderator')            AS moderators
		FROM users
	`
	var total, active, blocked, admins, moderators int
	if err := r.db.QueryRow(ctx, q).Scan(&total, &active, &blocked, &admins, &moderators); err != nil {
		return nil, err
	}
	return map[string]int{
		"total":      total,
		"active":     active,
		"blocked":    blocked,
		"admins":     admins,
		"moderators": moderators,
	}, nil
}

// TopUsersByDisk — left join so anonymous (guest) uploads still show up
// with email = ”. Limit hard-capped at 200 to keep the response small.
func (r *adminStatsRepo) TopUsersByDisk(ctx context.Context, limit int) ([]models.TopUserDisk, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	const q = `
		SELECT
			COALESCE(f.user_id::text, '')   AS user_id,
			COALESCE(u.email, '')           AS email,
			COUNT(*)                        AS file_count,
			COALESCE(SUM(f.file_size), 0)   AS total_size
		FROM files f
		LEFT JOIN users u ON u.id = f.user_id
		GROUP BY f.user_id, u.email
		ORDER BY total_size DESC
		LIMIT $1
	`
	rows, err := r.db.Query(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]models.TopUserDisk, 0, limit)
	for rows.Next() {
		var t models.TopUserDisk
		if err := rows.Scan(&t.UserID, &t.Email, &t.FileCount, &t.TotalSize); err != nil {
			r.log.Error("admin_stats: scan top disk", logger.Error(err))
			continue
		}
		out = append(out, t)
	}
	return out, nil
}
