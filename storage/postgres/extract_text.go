package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
)

type extractTextRepo struct {
	db  *pgxpool.Pool
	log logger.ILogger
}

func NewExtractTextRepo(db *pgxpool.Pool, log logger.ILogger) *extractTextRepo {
	return &extractTextRepo{db: db, log: log}
}

func (r *extractTextRepo) Create(ctx context.Context, job *models.ExtractTextJob) error {
	const q = `
		INSERT INTO extract_text_jobs
		    (id, user_id, input_file_id, format, status, created_at)
		VALUES ($1,$2,$3,$4,$5,$6)
	`
	_, err := r.db.Exec(ctx, q, job.ID, handleUserID(job.UserID),
		job.InputFileID, job.Format, job.Status, job.CreatedAt)
	if err != nil {
		r.log.Error("extract_text: create", logger.Error(err))
	}
	return err
}

func (r *extractTextRepo) GetByID(ctx context.Context, id string) (*models.ExtractTextJob, error) {
	const q = `
		SELECT id, user_id, input_file_id, format, output_file_id, status, created_at
		FROM extract_text_jobs WHERE id = $1
	`
	var (
		job        models.ExtractTextJob
		userID     sql.NullString
		outputFile sql.NullString
	)
	err := r.db.QueryRow(ctx, q, id).Scan(&job.ID, &userID, &job.InputFileID,
		&job.Format, &outputFile, &job.Status, &job.CreatedAt)
	if err != nil {
		return nil, err
	}
	if userID.Valid {
		job.UserID = &userID.String
	}
	if outputFile.Valid {
		job.OutputFileID = &outputFile.String
	}
	return &job, nil
}

func (r *extractTextRepo) Update(ctx context.Context, job *models.ExtractTextJob) error {
	const q = `UPDATE extract_text_jobs SET output_file_id=$1, status=$2 WHERE id=$3`
	var out sql.NullString
	if job.OutputFileID != nil {
		out = sql.NullString{String: *job.OutputFileID, Valid: true}
	}
	res, err := r.db.Exec(ctx, q, out, job.Status, job.ID)
	if err != nil {
		r.log.Error("extract_text: update", logger.Error(err))
		return err
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("extract_text: no rows for %s", job.ID)
	}
	return nil
}
