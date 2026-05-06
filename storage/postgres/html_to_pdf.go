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

type htmlToPDFRepo struct {
	db  *pgxpool.Pool
	log logger.ILogger
}

func NewHTMLToPDFRepo(db *pgxpool.Pool, log logger.ILogger) storage.IHTMLToPDFStorage {
	return &htmlToPDFRepo{db: db, log: log}
}

func (r *htmlToPDFRepo) Create(ctx context.Context, job *models.HTMLToPDFJob) error {
	query := `
		INSERT INTO html_to_pdf_jobs (
			id, user_id, html_content, output_file_id, status, created_at
		) VALUES ($1, $2, $3, $4, $5, $6);
	`

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
		job.ID, userID, job.HTMLContent,
		outputFileID, job.Status, job.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("htmlToPDFRepo.Create: %w", err)
	}
	return nil
}

func (r *htmlToPDFRepo) GetByID(ctx context.Context, id string) (*models.HTMLToPDFJob, error) {
	query := `
		SELECT id, user_id, html_content, output_file_id, status, created_at
		FROM html_to_pdf_jobs
		WHERE id = $1;
	`

	row := r.db.QueryRow(ctx, query, id)

	var (
		userID       sql.NullString
		outputFileID sql.NullString
		job          models.HTMLToPDFJob
	)

	err := row.Scan(&job.ID, &userID, &job.HTMLContent, &outputFileID, &job.Status, &job.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("htmlToPDFRepo.GetByID: not found")
		}
		return nil, fmt.Errorf("htmlToPDFRepo.GetByID: %w", err)
	}

	if userID.Valid {
		job.UserID = &userID.String
	}
	if outputFileID.Valid {
		job.OutputFileID = &outputFileID.String
	}

	return &job, nil
}

func (r *htmlToPDFRepo) Update(ctx context.Context, job *models.HTMLToPDFJob) error {
	query := `
		UPDATE html_to_pdf_jobs
		SET output_file_id = $1,
		    status = $2
		WHERE id = $3;
	`

	_, err := r.db.Exec(ctx, query, job.OutputFileID, job.Status, job.ID)
	if err != nil {
		return fmt.Errorf("htmlToPDFRepo.Update: %w", err)
	}
	return nil
}
