package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/storage"
)

// cropRepo implements ICropPDFStorage
type cropRepo struct {
	db  *pgxpool.Pool
	log logger.ILogger
}

// NewCropRepo konstruktor
func NewCropRepo(db *pgxpool.Pool, log logger.ILogger) storage.ICropPDFStorage {
	return &cropRepo{db: db, log: log}
}

func (r *cropRepo) Create(ctx context.Context, job *models.CropPDFJob) error {
	query := `
    INSERT INTO crop_pdf_jobs
    (id, user_id, input_file_id, top, bottom, "left", "right", status, created_at)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
`

	var userID interface{}
	if job.UserID != nil && *job.UserID != "" {
		userID = *job.UserID
	} else {
		userID = nil
	}

	_, err := r.db.Exec(ctx, query,
		job.ID, userID, job.InputFileID,
		job.Top, job.Bottom, job.Left, job.Right,
		job.Status, job.CreatedAt,
	)

	if err != nil {
		r.log.Error("cropRepo.Create failed", logger.Error(err))
		return fmt.Errorf("failed to create crop job: %w", err)
	}

	r.log.Info("Crop job successfully created", logger.String("jobID", job.ID))
	return nil
}

func (r *cropRepo) GetByID(ctx context.Context, id string) (*models.CropPDFJob, error) {
	query := `
    SELECT id, user_id, input_file_id, top, bottom, "left", "right",
           output_file_id, status, created_at
    FROM crop_pdf_jobs
    WHERE id = $1
`

	row := r.db.QueryRow(ctx, query, id)

	var job models.CropPDFJob
	err := row.Scan(
		&job.ID, &job.UserID, &job.InputFileID,
		&job.Top, &job.Bottom, &job.Left, &job.Right,
		&job.OutputFileID, &job.Status, &job.CreatedAt,
	)
	if err != nil {
		r.log.Error("cropRepo.GetByID failed", logger.Error(err))
		return nil, fmt.Errorf("failed to fetch crop job: %w", err)
	}

	// kerak bo‘lsa input fayl IDlarini qo‘shish (bu yerda bitta fayl)
	job.InputFileIDs = []string{job.InputFileID}

	return &job, nil
}

func (r *cropRepo) Update(ctx context.Context, job *models.CropPDFJob) error {
	query := `
        UPDATE crop_pdf_jobs
        SET output_file_id = $1, status = $2
        WHERE id = $3
    `
	_, err := r.db.Exec(ctx, query,
		job.OutputFileID, job.Status, job.ID,
	)

	if err != nil {
		r.log.Error("cropRepo.Update failed", logger.Error(err))
		return fmt.Errorf("failed to update crop job: %w", err)
	}

	r.log.Info("Crop job status updated successfully", logger.String("jobID", job.ID))
	return nil
}

// GetInputFiles – job uchun input fayllarni olish
func (r *cropRepo) GetInputFiles(ctx context.Context, jobID string) ([]string, error) {
	query := `SELECT file_id FROM crop_pdf_jobs WHERE job_id = $1`
	rows, err := r.db.Query(ctx, query, jobID)
	if err != nil {
		r.log.Error("failed to fetch input files", logger.Error(err))
		return nil, fmt.Errorf("failed to get input files for job %s: %w", jobID, err)
	}
	defer rows.Close()

	var fileIDs []string
	for rows.Next() {
		var fileID string
		if err := rows.Scan(&fileID); err != nil {
			r.log.Error("failed to scan file ID", logger.Error(err))
			return nil, err
		}
		fileIDs = append(fileIDs, fileID)
	}

	return fileIDs, nil
}
