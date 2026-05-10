package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
)

type urlToPDFRepo struct {
	db  *pgxpool.Pool
	log logger.ILogger
}

func NewURLToPDFRepo(db *pgxpool.Pool, log logger.ILogger) *urlToPDFRepo {
	return &urlToPDFRepo{db: db, log: log}
}

func (r *urlToPDFRepo) Create(ctx context.Context, job *models.URLToPDFJob) error {
	const q = `
		INSERT INTO url_to_pdf_jobs
		    (id, user_id, url, page_size, landscape, wait_for, status, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
	`
	var waitFor interface{}
	if job.WaitFor != nil && *job.WaitFor != "" {
		waitFor = *job.WaitFor
	}
	_, err := r.db.Exec(ctx, q, job.ID, handleUserID(job.UserID), job.URL,
		job.PageSize, job.Landscape, waitFor, job.Status, job.CreatedAt)
	if err != nil {
		r.log.Error("url_to_pdf: create", logger.Error(err))
	}
	return err
}

func (r *urlToPDFRepo) GetByID(ctx context.Context, id string) (*models.URLToPDFJob, error) {
	const q = `
		SELECT id, user_id, url, page_size, landscape, wait_for,
		       output_file_id, status, created_at
		FROM url_to_pdf_jobs WHERE id = $1
	`
	var (
		job        models.URLToPDFJob
		userID     sql.NullString
		waitFor    sql.NullString
		outputFile sql.NullString
	)
	err := r.db.QueryRow(ctx, q, id).Scan(&job.ID, &userID, &job.URL,
		&job.PageSize, &job.Landscape, &waitFor, &outputFile, &job.Status, &job.CreatedAt)
	if err != nil {
		return nil, err
	}
	if userID.Valid {
		job.UserID = &userID.String
	}
	if waitFor.Valid {
		job.WaitFor = &waitFor.String
	}
	if outputFile.Valid {
		job.OutputFileID = &outputFile.String
	}
	return &job, nil
}

func (r *urlToPDFRepo) Update(ctx context.Context, job *models.URLToPDFJob) error {
	const q = `UPDATE url_to_pdf_jobs SET output_file_id=$1, status=$2 WHERE id=$3`
	var out sql.NullString
	if job.OutputFileID != nil {
		out = sql.NullString{String: *job.OutputFileID, Valid: true}
	}
	res, err := r.db.Exec(ctx, q, out, job.Status, job.ID)
	if err != nil {
		r.log.Error("url_to_pdf: update", logger.Error(err))
		return err
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("url_to_pdf: no rows for %s", job.ID)
	}
	return nil
}
