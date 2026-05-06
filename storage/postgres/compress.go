package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
)

type compressRepo struct {
	db  *pgxpool.Pool
	log logger.ILogger
}

func NewCompressRepo(db *pgxpool.Pool, log logger.ILogger) *compressRepo {
	return &compressRepo{db: db, log: log}
}

// Foydalanuvchi ID ni tekshiruvchi yordamchi funksiya
func handleUserID(userID *string) interface{} {
	if userID != nil && *userID != "" {
		return *userID
	}
	return nil
}

func (r *compressRepo) Create(ctx context.Context, job *models.CompressJob) error {
	query := `
		INSERT INTO compress_jobs (id, user_id, input_file_id, compression, output_file_id, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	var userID = handleUserID(job.UserID)

	var outputFileID sql.NullString
	if job.OutputFileID != nil {
		outputFileID = sql.NullString{String: *job.OutputFileID, Valid: true}
	} else {
		outputFileID = sql.NullString{Valid: false}
	}

	_, err := r.db.Exec(ctx, query,
		job.ID,
		userID,
		job.InputFileID,
		job.CompressionLevel,
		outputFileID,
		job.Status,
		job.CreatedAt,
	)
	if err != nil {
		r.log.Error("Failed to create compress job", logger.Error(err))
		return err
	}
	return nil
}

func (r *compressRepo) GetByID(ctx context.Context, id string) (*models.CompressJob, error) {
	query := `
		SELECT id, user_id, input_file_id, compression, output_file_id, status, created_at
		FROM compress_jobs
		WHERE id = $1
	`

	var job models.CompressJob
	var userID sql.NullString
	var outputFileID sql.NullString

	err := r.db.QueryRow(ctx, query, id).Scan(
		&job.ID,
		&userID,
		&job.InputFileID,
		&job.CompressionLevel,
		&outputFileID,
		&job.Status,
		&job.CreatedAt,
	)
	if err != nil {
		r.log.Error("Failed to get compress job by ID", logger.Error(err))
		return nil, err
	}

	if userID.Valid {
		job.UserID = &userID.String
	}
	if outputFileID.Valid {
		job.OutputFileID = &outputFileID.String
	}

	return &job, nil
}

func (r *compressRepo) Update(ctx context.Context, job *models.CompressJob) error {
	query := `
		UPDATE compress_jobs
		SET output_file_id = $1, status = $2
		WHERE id = $3
	`

	var outputFileID sql.NullString
	if job.OutputFileID != nil {
		outputFileID = sql.NullString{String: *job.OutputFileID, Valid: true}
	} else {
		outputFileID = sql.NullString{Valid: false}
	}

	result, err := r.db.Exec(ctx, query, outputFileID, job.Status, job.ID)
	if err != nil {
		r.log.Error("Failed to update compress job", logger.Error(err))
		return err
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("no rows affected, job with id %s not found", job.ID)
	}

	return nil
}
