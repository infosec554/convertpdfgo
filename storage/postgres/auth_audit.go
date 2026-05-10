package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/storage"
)

type authAuditRepo struct {
	db  *pgxpool.Pool
	log logger.ILogger
}

func NewAuthAuditRepo(db *pgxpool.Pool, log logger.ILogger) storage.IAuthAuditStorage {
	return &authAuditRepo{db: db, log: log}
}

func (r *authAuditRepo) Record(ctx context.Context, e models.AuthAudit) error {
	id := e.ID
	if id == "" {
		id = uuid.NewString()
	}
	const q = `
		INSERT INTO auth_audit (id, user_id, email, event, provider, ip, user_agent)
		VALUES ($1, $2, NULLIF($3,''), $4, NULLIF($5,''), NULLIF($6,''), NULLIF($7,''))
	`
	_, err := r.db.Exec(ctx, q, id, e.UserID, e.Email, e.Event, e.Provider, e.IP, e.UserAgent)
	if err != nil {
		// Audit failures must not break login. Log + swallow.
		r.log.Error("auth_audit: insert", logger.Error(err), logger.String("event", e.Event))
		return err
	}
	return nil
}

func (r *authAuditRepo) ListByUser(ctx context.Context, userID string, limit int) ([]models.AuthAudit, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	const q = `
		SELECT id, user_id, COALESCE(email,''), event, COALESCE(provider,''),
		       COALESCE(ip,''), COALESCE(user_agent,''), created_at
		FROM auth_audit
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`
	rows, err := r.db.Query(ctx, q, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.AuthAudit
	for rows.Next() {
		var a models.AuthAudit
		if err := rows.Scan(&a.ID, &a.UserID, &a.Email, &a.Event, &a.Provider, &a.IP, &a.UserAgent, &a.CreatedAt); err != nil {
			r.log.Error("auth_audit: scan", logger.Error(err))
			continue
		}
		out = append(out, a)
	}
	return out, nil
}

// ListRecent returns the most recent audit events across every user.
// Hits idx_auth_audit_created (index-only descent).
func (r *authAuditRepo) ListRecent(ctx context.Context, limit int) ([]models.AuthAudit, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	const q = `
		SELECT id, user_id, COALESCE(email,''), event, COALESCE(provider,''),
		       COALESCE(ip,''), COALESCE(user_agent,''), created_at
		FROM auth_audit
		ORDER BY created_at DESC
		LIMIT $1
	`
	rows, err := r.db.Query(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.AuthAudit
	for rows.Next() {
		var a models.AuthAudit
		if err := rows.Scan(&a.ID, &a.UserID, &a.Email, &a.Event, &a.Provider, &a.IP, &a.UserAgent, &a.CreatedAt); err != nil {
			r.log.Error("auth_audit: scan recent", logger.Error(err))
			continue
		}
		out = append(out, a)
	}
	return out, nil
}

// ListSuspicious finds emails with `minFails`+ login_fail events in the
// last `withinMinutes`. We aggregate IPs and the most-recent timestamp so
// the admin can spot password-spray (one email, many IPs) vs. a single
// confused user (one IP, many fails).
func (r *authAuditRepo) ListSuspicious(ctx context.Context, withinMinutes, minFails int) ([]models.SuspiciousLogin, error) {
	if withinMinutes <= 0 {
		withinMinutes = 60
	}
	if minFails <= 0 {
		minFails = 5
	}
	const q = `
		SELECT
			COALESCE(email, '')                       AS email,
			MAX(user_id::text)                        AS user_id,
			COUNT(*)                                  AS fails,
			ARRAY_AGG(DISTINCT COALESCE(ip,''))       AS ips,
			MAX(created_at)                           AS last_seen
		FROM auth_audit
		WHERE event = 'login_fail'
		  AND created_at >= NOW() - ($1::int || ' minutes')::interval
		  AND COALESCE(email, '') <> ''
		GROUP BY email
		HAVING COUNT(*) >= $2::int
		ORDER BY fails DESC, last_seen DESC
		LIMIT 100
	`
	rows, err := r.db.Query(ctx, q, withinMinutes, minFails)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.SuspiciousLogin
	for rows.Next() {
		var s models.SuspiciousLogin
		var uid *string
		if err := rows.Scan(&s.Email, &uid, &s.FailCount, &s.IPs, &s.LastSeen); err != nil {
			r.log.Error("auth_audit: scan suspicious", logger.Error(err))
			continue
		}
		if uid != nil && *uid != "" {
			s.UserID = uid
		}
		out = append(out, s)
	}
	return out, nil
}
