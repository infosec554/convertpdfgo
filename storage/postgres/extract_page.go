package postgres

import (
	"context"
	"database/sql"

	"github.com/jackc/pgx/v5/pgxpool"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/storage"
)

type extractRepo struct {
	db  *pgxpool.Pool
	log logger.ILogger
}

func NewExtractPageRepo(db *pgxpool.Pool, log logger.ILogger) storage.IExtractPageStorage {
	return &extractRepo{
		db:  db,
		log: log,
	}
}
func (r *extractRepo) Create(ctx context.Context, job *models.ExtractJob) error {
	query := `
        INSERT INTO extract_pages_jobs (id, user_id, input_file_id, pages_to_extract, output_file_id, status, created_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
    `

	var userID interface{}
	if job.UserID != nil && *job.UserID != "" {
		userID = *job.UserID
	} else {
		userID = nil // NULL for guest users
	}

	// Check for nil OutputFileID and use sql.NullString accordingly
	var outputFileID sql.NullString
	if job.OutputFileID != nil {
		outputFileID = sql.NullString{String: *job.OutputFileID, Valid: true}
	} else {
		outputFileID = sql.NullString{Valid: false} // NULL value in DB
	}
	_, err := r.db.Exec(ctx, query,
		job.ID, userID, job.InputFileID, job.PagesToExtract, outputFileID, job.Status, job.CreatedAt)
	return err
}

func (r *extractRepo) Update(ctx context.Context, job *models.ExtractJob) error {
	query := `
		UPDATE extract_pages_jobs
		SET output_file_id = $1, status = $2
		WHERE id = $3
	`
	_, err := r.db.Exec(ctx, query, job.OutputFileID, job.Status, job.ID)
	return err
}

func (r *extractRepo) GetByID(ctx context.Context, id string) (*models.ExtractJob, error) {
	query := `
		SELECT id, user_id, input_file_id, pages_to_extract, output_file_id, status, created_at
		FROM extract_pages_jobs
		WHERE id = $1
	`

	var job models.ExtractJob
	var outputFileID *string

	err := r.db.QueryRow(ctx, query, id).Scan(
		&job.ID,
		&job.UserID,
		&job.InputFileID,
		&job.PagesToExtract,
		&outputFileID,
		&job.Status,
		&job.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	job.OutputFileID = outputFileID // ✅ NULL bo‘lsa nil bo‘ladi

	return &job, nil
}
