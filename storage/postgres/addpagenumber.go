package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
)

type addPageNumberRepo struct {
	db  *pgxpool.Pool
	log logger.ILogger
}

func NewAddPageNumberRepo(db *pgxpool.Pool, log logger.ILogger) *addPageNumberRepo {
	return &addPageNumberRepo{db: db, log: log}
}

func (r *addPageNumberRepo) Create(ctx context.Context, job *models.AddPageNumberJob) error {
	query := `
	INSERT INTO add_page_number_jobs (
		id, user_id, input_file_id, status,
		created_at, first_number, page_range,
		position, color, font_size
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	var userID interface{}
	if job.UserID != nil && *job.UserID != "" {
		userID = *job.UserID
	} else {
		userID = nil
	}

	_, err := r.db.Exec(ctx, query,
		job.ID, userID, job.InputFileID, job.Status,
		job.CreatedAt, job.FirstNumber, job.PageRange,
		job.Position, job.Color, job.FontSize,
	)

	if err != nil {
		r.log.Error("failed to insert AddPageNumberJob", logger.Error(err))
	}
	return err
}

func (r *addPageNumberRepo) GetByID(ctx context.Context, id string) (*models.AddPageNumberJob, error) {
	query := `
		SELECT id, user_id, input_file_id, output_file_id,
		       status, created_at, first_number, page_range,
		       position, color, font_size
		FROM add_page_number_jobs WHERE id = $1
	`

	var job models.AddPageNumberJob
	err := r.db.QueryRow(ctx, query, id).Scan(
		&job.ID, &job.UserID, &job.InputFileID, &job.OutputFileID,
		&job.Status, &job.CreatedAt, &job.FirstNumber, &job.PageRange,
		&job.Position, &job.Color, &job.FontSize,
	)
	if err != nil {
		r.log.Error("failed to get AddPageNumberJob", logger.Error(err))
		return nil, err
	}

	return &job, nil
}

func (r *addPageNumberRepo) Update(ctx context.Context, job *models.AddPageNumberJob) error {
	query := `
		UPDATE add_page_number_jobs
		SET output_file_id = $1, status = $2
		WHERE id = $3
	`
	_, err := r.db.Exec(ctx, query, job.OutputFileID, job.Status, job.ID)
	if err != nil {
		r.log.Error("failed to update AddPageNumberJob", logger.Error(err))
	}
	return err
}
