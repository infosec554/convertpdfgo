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

type excelToPDFRepo struct {
	db  *pgxpool.Pool
	log logger.ILogger
}

func NewExcelToPDFRepo(db *pgxpool.Pool, log logger.ILogger) storage.IExcelToPDFStorage {
	return &excelToPDFRepo{db: db, log: log}
}

func (r *excelToPDFRepo) Create(ctx context.Context, job *models.ExcelToPDFJob) error {
	query := `
        INSERT INTO excel_to_pdf_jobs (
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
		job.ID, userID, job.InputFileID, outputFileID, job.Status, job.CreatedAt)

	if err != nil {
		return fmt.Errorf("excelToPDFRepo.Create: %w", err)
	}
	return nil
}

func (r *excelToPDFRepo) GetByID(ctx context.Context, id string) (*models.ExcelToPDFJob, error) {
	query := `
        SELECT id, user_id, input_file_id, output_file_id, status, created_at
        FROM excel_to_pdf_jobs
        WHERE id = $1;`

	row := r.db.QueryRow(ctx, query, id)

	var (
		userID       sql.NullString
		outputFileID sql.NullString
		job          models.ExcelToPDFJob
	)

	err := row.Scan(&job.ID, &userID, &job.InputFileID, &outputFileID, &job.Status, &job.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("excelToPDFRepo.GetByID: not found")
		}
		return nil, fmt.Errorf("excelToPDFRepo.GetByID: %w", err)
	}

	if userID.Valid {
		job.UserID = &userID.String
	}
	if outputFileID.Valid {
		job.OutputFileID = &outputFileID.String
	}

	return &job, nil
}

func (r *excelToPDFRepo) Update(ctx context.Context, job *models.ExcelToPDFJob) error {
	query := `
        UPDATE excel_to_pdf_jobs
        SET output_file_id = $1,
            status = $2
        WHERE id = $3;
    `

	_, err := r.db.Exec(ctx, query,
		job.OutputFileID, job.Status, job.ID)

	if err != nil {
		return fmt.Errorf("excelToPDFRepo.Update: %w", err)
	}

	return nil
}
