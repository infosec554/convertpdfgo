package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/storage"
)

type detectBlankPagesRepo struct {
	db  *pgxpool.Pool
	log logger.ILogger
}

func NewDetectBlankPagesRepo(db *pgxpool.Pool, log logger.ILogger) storage.IDetectBlankPagesStorage {
	return &detectBlankPagesRepo{db: db, log: log}
}

func (r *detectBlankPagesRepo) Create(ctx context.Context, job *models.DetectBlankPagesJob) error {
	query := `
		INSERT INTO detect_blank_pages_jobs (id, user_id, input_file_id, blank_pages, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	_, err := r.db.Exec(ctx, query,
		job.ID,
		job.UserID,
		job.InputFileID,
		job.BlankPages,
		job.Status,
		time.Now(),
	)

	if err != nil {
		r.log.Error("failed to create detect blank pages job", logger.Error(err))
	}

	return err
}

func (r *detectBlankPagesRepo) Update(ctx context.Context, job *models.DetectBlankPagesJob) error {
	query := `
		UPDATE detect_blank_pages_jobs
		SET blank_pages = $1, status = $2
		WHERE id = $3
	`

	_, err := r.db.Exec(ctx, query,
		job.BlankPages,
		job.Status,
		job.ID,
	)

	if err != nil {
		r.log.Error("failed to update detect blank pages job", logger.Error(err))
	}

	return err
}

func (r *detectBlankPagesRepo) GetByID(ctx context.Context, id string) (*models.DetectBlankPagesJob, error) {
	query := `
		SELECT id, user_id, input_file_id, blank_pages, status, created_at
		FROM detect_blank_pages_jobs
		WHERE id = $1
	`

	var job models.DetectBlankPagesJob
	err := r.db.QueryRow(ctx, query, id).Scan(
		&job.ID,
		&job.UserID,
		&job.InputFileID,
		&job.BlankPages,
		&job.Status,
		&job.CreatedAt,
	)
	if err != nil {
		r.log.Error("failed to get detect blank pages job", logger.Error(err))
		return nil, err
	}

	return &job, nil
}
