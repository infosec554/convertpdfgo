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
