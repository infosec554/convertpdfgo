package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
)

type rotateRepo struct {
	db  *pgxpool.Pool
	log logger.ILogger
}

// NewRotateRepo initializes the rotate repository
func NewRotateRepo(db *pgxpool.Pool, log logger.ILogger) *rotateRepo {
	return &rotateRepo{db: db, log: log}
}
func (r *rotateRepo) Create(ctx context.Context, job *models.RotateJob) error {
	query := `
		INSERT INTO rotate_jobs (
			id, user_id, input_file_id, rotation_angle, pages,
			output_file_id, output_path, status, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	var userID interface{}
	if job.UserID != nil && *job.UserID != "" {
		userID = *job.UserID
	} else {
		userID = nil
	}

	// DOIM output_file_id NULL bo‘ladi create bosqichida
	var outputFileID interface{} = nil

	_, err := r.db.Exec(ctx, query,
		job.ID,
		userID,
		job.InputFileID,
		job.Angle,
		job.Pages,
		outputFileID, // ✅ NULL yuboriladi
		job.OutputPath,
		job.Status,
		job.CreatedAt,
	)

	if err != nil {
		r.log.Error("rotateRepo.Create query error", logger.Error(err))
		return err
	}
	return nil
}

// GetByID retrieves a rotate job by its ID
func (r *rotateRepo) GetByID(ctx context.Context, id string) (*models.RotateJob, error) {
	// SQL Query to fetch job by ID
	query := `
		SELECT id, user_id, input_file_id, rotation_angle, pages, output_file_id, output_path, status, created_at
		FROM rotate_jobs
		WHERE id = $1
	`

	var job models.RotateJob
	err := r.db.QueryRow(ctx, query, id).Scan(
		&job.ID,
		&job.UserID,
		&job.InputFileID,
		&job.Angle,
		&job.Pages,
		&job.OutputFileID,
		&job.OutputPath,
		&job.Status,
		&job.CreatedAt,
	)
	if err != nil {
		r.log.Error("rotateRepo.GetByID query error", logger.Error(err))
		return nil, err
	}

	// Logging for guest users
	if job.UserID == nil {
		r.log.Info("Guest user accessed rotate job", logger.String("jobID", job.ID))
	}

	return &job, nil
}

// Update updates the status and file details of a rotate job
func (r *rotateRepo) Update(ctx context.Context, job *models.RotateJob) error {
	query := `
		UPDATE rotate_jobs
		SET output_file_id = $1, output_path = $2, status = $3
		WHERE id = $4
	`

	_, err := r.db.Exec(ctx, query,
		job.OutputFileID, // ✅ Bu paytda allaqachon files ga saqlangan ID bo‘ladi
		job.OutputPath,
		job.Status,
		job.ID,
	)
	return err
}
