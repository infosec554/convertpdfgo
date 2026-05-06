package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/storage"
)

type inspectRepo struct {
	db  *pgxpool.Pool
	log logger.ILogger
}

func NewInspectRepo(db *pgxpool.Pool, log logger.ILogger) storage.IInspectStorage {
	return &inspectRepo{db: db, log: log}
}

func (r *inspectRepo) Create(ctx context.Context, job *models.InspectJob) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO pdf_inspect_jobs 
		(id, user_id, file_id, page_count, title, author, subject, keywords, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`,
		job.ID, job.UserID, job.FileID,
		job.PageCount, job.Title, job.Author, job.Subject, job.Keywords,
		job.Status, job.CreatedAt,
	)
	return err
}

func (r *inspectRepo) GetByID(ctx context.Context, id string) (*models.InspectJob, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, user_id, file_id, page_count, title, author, subject, keywords, status, created_at
		FROM pdf_inspect_jobs WHERE id = $1
	`, id)

	var job models.InspectJob
	err := row.Scan(
		&job.ID, &job.UserID, &job.FileID, &job.PageCount,
		&job.Title, &job.Author, &job.Subject, &job.Keywords,
		&job.Status, &job.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &job, nil
}
