package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/storage"
)

type sharedLinkRepo struct {
	db  *pgxpool.Pool
	log logger.ILogger
}

func NewSharedLinkRepo(db *pgxpool.Pool, log logger.ILogger) storage.ISharedLinkStorage {
	return &sharedLinkRepo{db: db, log: log}
}

func (r *sharedLinkRepo) Create(ctx context.Context, req *models.SharedLink) error {
	// SQL so'rovini tuzish
	query := `
		INSERT INTO shared_links (id, file_id, shared_token, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`

	// Xatolikni aniqlash va loglash
	_, err := r.db.Exec(ctx, query,
		req.ID,
		req.FileID,
		req.SharedToken,
		req.ExpiresAt,
		req.CreatedAt,
	)
	if err != nil {
		r.log.Error("failed to insert shared link", logger.Error(err))
		return fmt.Errorf("failed to create shared link: %w", err)
	}

	r.log.Info("shared link created", logger.String("file_id", req.FileID), logger.String("shared_token", req.SharedToken))
	return nil
}

func (r *sharedLinkRepo) GetByToken(ctx context.Context, token string) (*models.SharedLink, error) {
	// SQL so'rovini tuzish
	query := `
		SELECT id, file_id, shared_token, expires_at, created_at
		FROM shared_links
		WHERE shared_token = $1
	`

	// So'rovni bajarish va natijalarni olish
	var link models.SharedLink
	err := r.db.QueryRow(ctx, query, token).Scan(
		&link.ID,
		&link.FileID,
		&link.SharedToken,
		&link.ExpiresAt,
		&link.CreatedAt,
	)

	// Xatolikni aniqlash va loglash
	if err != nil {
		if err.Error() == "no rows in result set" {
			// Agar token bo'yicha natija topilmasa, aniq xatolik qaytarish
			r.log.Error("shared link not found", logger.String("token", token))
			return nil, fmt.Errorf("shared link not found for token: %s", token)
		}
		r.log.Error("failed to get shared link by token", logger.Error(err))
		return nil, fmt.Errorf("failed to get shared link by token: %w", err)
	}

	r.log.Info("retrieved shared link", logger.String("token", token))
	return &link, nil
}
