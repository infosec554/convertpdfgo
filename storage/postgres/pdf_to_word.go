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

type pdfToWordRepo struct {
	db  *pgxpool.Pool
	log logger.ILogger
}

func NewPDFToWordRepo(db *pgxpool.Pool, log logger.ILogger) storage.IPDFToWordStorage {
	return &pdfToWordRepo{db: db, log: log}
}

func (r *pdfToWordRepo) Create(ctx context.Context, job *models.PDFToWordJob) error {
	query := `
		INSERT INTO pdf_to_word_jobs (
			id, user_id, input_file_id, output_file_id, status, created_at
		) VALUES ($1, $2, $3, $4, $5, $6);`

	var userID interface{}
	if job.UserID != nil && *job.UserID != "" {
		userID = *job.UserID
	} else {
		userID = nil
	}

	var outputFileID sql.NullString
	if job.OutputFileID != nil {
		outputFileID = sql.NullString{String: *job.OutputFileID, Valid: true}
	} else {
		outputFileID = sql.NullString{Valid: false}
	}

	_, err := r.db.Exec(ctx, query,
		job.ID, userID, job.InputFileID,
		outputFileID, job.Status, job.CreatedAt)

	if err != nil {
		return fmt.Errorf("pdfToWordRepo.Create: %w", err)
	}
	return nil
}

func (r *pdfToWordRepo) GetByID(ctx context.Context, id string) (*models.PDFToWordJob, error) {
	query := `
		SELECT id, user_id, input_file_id, output_file_id, status, created_at
		FROM pdf_to_word_jobs
		WHERE id = $1;
	`

	row := r.db.QueryRow(ctx, query, id)
	var job models.PDFToWordJob
	err := row.Scan(&job.ID, &job.UserID, &job.InputFileID,
		&job.OutputFileID, &job.Status, &job.CreatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("pdfToWordRepo.GetByID: not found")
		}
		return nil, fmt.Errorf("pdfToWordRepo.GetByID: %w", err)
	}

	return &job, nil
}

func (r *pdfToWordRepo) Update(ctx context.Context, job *models.PDFToWordJob) error {
	query := `
		UPDATE pdf_to_word_jobs
		SET output_file_id = $1,
			status = $2
		WHERE id = $3;
	`

	_, err := r.db.Exec(ctx, query,
		job.OutputFileID, job.Status, job.ID)

	if err != nil {
		return fmt.Errorf("pdfToWordRepo.Update: %w", err)
	}

	return nil
}
