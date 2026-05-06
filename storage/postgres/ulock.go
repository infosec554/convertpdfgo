package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/storage"
)

type unlockRepo struct {
	db  *pgxpool.Pool
	log logger.ILogger
}

func NewUnlockRepo(db *pgxpool.Pool, log logger.ILogger) storage.IUnlockPDFStorage {
	return &unlockRepo{db: db, log: log}
}

func (r *unlockRepo) Create(ctx context.Context, job *models.UnlockPDFJob) error {
	query := `
		INSERT INTO unlock_jobs (
			id, user_id, input_file_id, output_file_id, status, created_at
		) VALUES ($1, $2, $3, $4, $5, $6);`

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
		job.ID, userID, job.InputFileID,
		outputFileID, job.Status, job.CreatedAt)

	if err != nil {
		return fmt.Errorf("unlockRepo.Create: %w", err)
	}

	return nil
}

func (r *unlockRepo) GetByID(ctx context.Context, id string) (*models.UnlockPDFJob, error) {
	query := `
		SELECT id, user_id, input_file_id, output_file_id, status, created_at
		FROM unlock_jobs
		WHERE id = $1;
	`

	row := r.db.QueryRow(ctx, query, id)

	var job models.UnlockPDFJob
	err := row.Scan(
		&job.ID, &job.UserID, &job.InputFileID,
		&job.OutputFileID, &job.Status, &job.CreatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("unlockRepo.GetByID: not found")
		}
		return nil, fmt.Errorf("unlockRepo.GetByID: %w", err)
	}

	return &job, nil
}

func (r *unlockRepo) Update(ctx context.Context, job *models.UnlockPDFJob) error {
	query := `
		UPDATE unlock_jobs
		SET output_file_id = $1,
			status = $2
		WHERE id = $3;
	`

	_, err := r.db.Exec(ctx, query,
		job.OutputFileID, job.Status, job.ID)

	if err != nil {
		return fmt.Errorf("unlockRepo.Update: %w", err)
	}

	return nil
}
