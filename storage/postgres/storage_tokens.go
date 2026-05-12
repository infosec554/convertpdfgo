package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/storage"
)

type storageTokenRepo struct {
	db  *pgxpool.Pool
	log logger.ILogger
}

func NewStorageTokenRepo(db *pgxpool.Pool, log logger.ILogger) storage.IStorageTokenStorage {
	return &storageTokenRepo{db: db, log: log}
}

// Upsert writes (or replaces) the token row for (user_id, provider).
// The unique index on those two columns is what makes this idempotent —
// reconnecting a provider just refreshes the credentials in place.
func (r *storageTokenRepo) Upsert(ctx context.Context, t *models.UserStorageToken) error {
	if t.ID == "" {
		t.ID = uuid.NewString()
	}
	const q = `
		INSERT INTO user_storage_tokens
		    (id, user_id, provider, access_token, refresh_token,
		     expires_at, scope, account_email, created_at, updated_at)
		VALUES ($1,$2,$3,$4,NULLIF($5,''),$6,NULLIF($7,''),NULLIF($8,''), NOW(), NOW())
		ON CONFLICT (user_id, provider) DO UPDATE SET
		    access_token  = EXCLUDED.access_token,
		    refresh_token = COALESCE(EXCLUDED.refresh_token, user_storage_tokens.refresh_token),
		    expires_at    = EXCLUDED.expires_at,
		    scope         = COALESCE(EXCLUDED.scope, user_storage_tokens.scope),
		    account_email = COALESCE(EXCLUDED.account_email, user_storage_tokens.account_email),
		    updated_at    = NOW()
	`
	var expiresAt interface{}
	if t.ExpiresAt != nil {
		expiresAt = *t.ExpiresAt
	}
	_, err := r.db.Exec(ctx, q,
		t.ID, t.UserID, t.Provider,
		t.AccessToken, t.RefreshToken,
		expiresAt, t.Scope, t.AccountEmail,
	)
	if err != nil {
		return fmt.Errorf("storage_tokens.Upsert: %w", err)
	}
	return nil
}

func (r *storageTokenRepo) Get(ctx context.Context, userID, provider string) (*models.UserStorageToken, error) {
	const q = `
		SELECT id, user_id, provider, access_token,
		       COALESCE(refresh_token,''), expires_at,
		       COALESCE(scope,''), COALESCE(account_email,''),
		       created_at, updated_at
		FROM user_storage_tokens
		WHERE user_id = $1 AND provider = $2
	`
	var t models.UserStorageToken
	var expiresAt sql.NullTime
	err := r.db.QueryRow(ctx, q, userID, provider).Scan(
		&t.ID, &t.UserID, &t.Provider, &t.AccessToken,
		&t.RefreshToken, &expiresAt,
		&t.Scope, &t.AccountEmail,
		&t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("storage_tokens.Get: not found")
		}
		return nil, fmt.Errorf("storage_tokens.Get: %w", err)
	}
	if expiresAt.Valid {
		t.ExpiresAt = &expiresAt.Time
	}
	return &t, nil
}

// ListByUser returns every connected provider for the user, used by
// GET /api/storage/providers to drive the dropdown.
func (r *storageTokenRepo) ListByUser(ctx context.Context, userID string) ([]models.UserStorageToken, error) {
	const q = `
		SELECT id, user_id, provider, access_token,
		       COALESCE(refresh_token,''), expires_at,
		       COALESCE(scope,''), COALESCE(account_email,''),
		       created_at, updated_at
		FROM user_storage_tokens
		WHERE user_id = $1
		ORDER BY provider
	`
	rows, err := r.db.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("storage_tokens.ListByUser: %w", err)
	}
	defer rows.Close()
	var out []models.UserStorageToken
	for rows.Next() {
		var t models.UserStorageToken
		var expiresAt sql.NullTime
		if err := rows.Scan(
			&t.ID, &t.UserID, &t.Provider, &t.AccessToken,
			&t.RefreshToken, &expiresAt,
			&t.Scope, &t.AccountEmail,
			&t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			r.log.Error("storage_tokens.ListByUser: scan", logger.Error(err))
			continue
		}
		if expiresAt.Valid {
			t.ExpiresAt = &expiresAt.Time
		}
		out = append(out, t)
	}
	return out, nil
}

// Delete drops the token row (user has clicked "Disconnect Dropbox"
// or similar). Missing rows are not an error.
func (r *storageTokenRepo) Delete(ctx context.Context, userID, provider string) error {
	const q = `DELETE FROM user_storage_tokens WHERE user_id = $1 AND provider = $2`
	_, err := r.db.Exec(ctx, q, userID, provider)
	if err != nil {
		return fmt.Errorf("storage_tokens.Delete: %w", err)
	}
	return nil
}

// UpdateAccessOnly is the cheap path the refresh-token flow takes:
// keep the refresh_token, replace only access_token + expires_at.
func (r *storageTokenRepo) UpdateAccessOnly(ctx context.Context, userID, provider, accessToken string, expiresAt *time.Time) error {
	const q = `
		UPDATE user_storage_tokens
		SET access_token = $3, expires_at = $4, updated_at = NOW()
		WHERE user_id = $1 AND provider = $2
	`
	var ea interface{}
	if expiresAt != nil {
		ea = *expiresAt
	}
	_, err := r.db.Exec(ctx, q, userID, provider, accessToken, ea)
	if err != nil {
		return fmt.Errorf("storage_tokens.UpdateAccessOnly: %w", err)
	}
	return nil
}
