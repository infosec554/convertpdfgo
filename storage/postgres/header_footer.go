package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/storage"
)

type addHeaderFooterRepo struct {
	db  *pgxpool.Pool
	log logger.ILogger
}

func NewAddHeaderFooterRepo(db *pgxpool.Pool, log logger.ILogger) storage.AddHeaderFooterStorage {
	return &addHeaderFooterRepo{db: db, log: log}
}

func (r *addHeaderFooterRepo) Create(ctx context.Context, job *models.AddHeaderFooterJob) error {
	query := `
		INSERT INTO add_header_footer_jobs (
			id, user_id, input_file_id, header_text, footer_text, font_size, font_color, position, status, created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
	`

	_, err := r.db.Exec(ctx, query,
		job.ID,
		job.UserID,
		job.InputFileID,
		job.HeaderText,
		job.FooterText,
		job.FontSize,
		job.FontColor,
		job.Position,
		job.Status,
		job.CreatedAt,
	)

	return err
}

func (r *addHeaderFooterRepo) Update(ctx context.Context, job *models.AddHeaderFooterJob) error {
	query := `
		UPDATE add_header_footer_jobs
		SET output_file_id=$1, status=$2
		WHERE id=$3
	`

	_, err := r.db.Exec(ctx, query,
		job.OutputFileID,
		job.Status,
		job.ID,
	)

	return err
}

func (r *addHeaderFooterRepo) GetByID(ctx context.Context, id string) (*models.AddHeaderFooterJob, error) {
	var job models.AddHeaderFooterJob

	query := `
		SELECT id, user_id, input_file_id, header_text, footer_text, font_size, font_color, position,
		       output_file_id, status, created_at
		FROM add_header_footer_jobs
		WHERE id = $1
	`

	err := r.db.QueryRow(ctx, query, id).Scan(
		&job.ID,
		&job.UserID,
		&job.InputFileID,
		&job.HeaderText,
		&job.FooterText,
		&job.FontSize,
		&job.FontColor,
		&job.Position,
		&job.OutputFileID,
		&job.Status,
		&job.CreatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("get header/footer job by id: %w", err)
	}

	return &job, nil
}
