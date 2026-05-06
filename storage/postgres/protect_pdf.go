package postgres

import (
	"context"
	"database/sql"

	"github.com/jackc/pgx/v5/pgxpool"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
)

type protectRepo struct {
	db  *pgxpool.Pool
	log logger.ILogger
}

func NewProtectRepo(db *pgxpool.Pool, log logger.ILogger) *protectRepo {
	return &protectRepo{
		db:  db,
		log: log,
	}
}

func (r *protectRepo) Create(ctx context.Context, job *models.ProtectPDFJob) error {
	query := `
        INSERT INTO protect_jobs (
            id, user_id, input_file_id, output_file_id, password, status, created_at
        ) VALUES ($1, $2, $3, $4, $5, $6, $7)
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
		job.ID,
		userID,
		job.InputFileID,
		outputFileID,
		job.Password,
		job.Status,
		job.CreatedAt,
	)
	return err
}

func (r *protectRepo) GetByID(ctx context.Context, id string) (*models.ProtectPDFJob, error) {
	query := `
        SELECT id, user_id, input_file_id, output_file_id, password, status, created_at
        FROM protect_jobs
        WHERE id = $1
    `
	var job models.ProtectPDFJob
	err := r.db.QueryRow(ctx, query, id).Scan(
		&job.ID,
		&job.UserID,
		&job.InputFileID,
		&job.OutputFileID,
		&job.Password,
		&job.Status,
		&job.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func (r *protectRepo) Update(ctx context.Context, job *models.ProtectPDFJob) error {
	query := `
        UPDATE protect_jobs
        SET status = $1, output_file_id = $2
        WHERE id = $3
    `

	var outputFileID interface{}
	if job.OutputFileID != nil && *job.OutputFileID != "" {
		outputFileID = *job.OutputFileID
	} else {
		outputFileID = nil
	}

	_, err := r.db.Exec(ctx, query, job.Status, outputFileID, job.ID)
	return err
}
