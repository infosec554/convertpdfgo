package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
)

type watermarkRepo struct {
	db  *pgxpool.Pool
	log logger.ILogger
}

func NewWatermarkRepo(db *pgxpool.Pool, log logger.ILogger) *watermarkRepo {
	return &watermarkRepo{db: db, log: log}
}

func (r *watermarkRepo) Create(ctx context.Context, job *models.WatermarkJob) error {
	const q = `
		INSERT INTO watermark_jobs
		    (id, user_id, input_file_id, mode, watermark_text, image_file_id,
		     position, rotation, opacity, pages, status, created_at)
		VALUES ($1,$2,$3,$4,NULLIF($5,''),$6,$7,$8,$9,$10,$11,$12)
	`
	var imgID interface{}
	if job.ImageFileID != nil && *job.ImageFileID != "" {
		imgID = *job.ImageFileID
	}
	_, err := r.db.Exec(ctx, q,
		job.ID, handleUserID(job.UserID), job.InputFileID, job.Mode, job.Text,
		imgID, job.Position, job.Rotation, job.Opacity, job.Pages,
		job.Status, job.CreatedAt,
	)
	if err != nil {
		r.log.Error("watermark: create", logger.Error(err))
	}
	return err
}

func (r *watermarkRepo) GetByID(ctx context.Context, id string) (*models.WatermarkJob, error) {
	const q = `
		SELECT id, user_id, input_file_id, mode, COALESCE(watermark_text,''),
		       image_file_id, COALESCE(position,''), rotation, opacity, pages,
		       output_file_id, status, created_at
		FROM watermark_jobs
		WHERE id = $1
	`
	var (
		job        models.WatermarkJob
		userID     sql.NullString
		imgID      sql.NullString
		outputFile sql.NullString
	)
	err := r.db.QueryRow(ctx, q, id).Scan(
		&job.ID, &userID, &job.InputFileID, &job.Mode, &job.Text,
		&imgID, &job.Position, &job.Rotation, &job.Opacity, &job.Pages,
		&outputFile, &job.Status, &job.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if userID.Valid {
		job.UserID = &userID.String
	}
	if imgID.Valid {
		job.ImageFileID = &imgID.String
	}
	if outputFile.Valid {
		job.OutputFileID = &outputFile.String
	}
	return &job, nil
}

func (r *watermarkRepo) Update(ctx context.Context, job *models.WatermarkJob) error {
	const q = `
		UPDATE watermark_jobs SET output_file_id = $1, status = $2 WHERE id = $3
	`
	var out sql.NullString
	if job.OutputFileID != nil {
		out = sql.NullString{String: *job.OutputFileID, Valid: true}
	}
	res, err := r.db.Exec(ctx, q, out, job.Status, job.ID)
	if err != nil {
		r.log.Error("watermark: update", logger.Error(err))
		return err
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("watermark: no rows for %s", job.ID)
	}
	return nil
}
