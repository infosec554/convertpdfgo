package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
)

type signRepo struct {
	db  *pgxpool.Pool
	log logger.ILogger
}

func NewSignRepo(db *pgxpool.Pool, log logger.ILogger) *signRepo {
	return &signRepo{db: db, log: log}
}

func (r *signRepo) Create(ctx context.Context, job *models.SignJob) error {
	const q = `
		INSERT INTO sign_jobs
		    (id, user_id, input_file_id, signature_file_id,
		     pages, x, y, width, status, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
	`
	_, err := r.db.Exec(ctx, q,
		job.ID, handleUserID(job.UserID), job.InputFileID, job.SignatureFileID,
		job.Pages, job.X, job.Y, job.Width,
		job.Status, job.CreatedAt,
	)
	if err != nil {
		r.log.Error("sign: create", logger.Error(err))
	}
	return err
}

func (r *signRepo) GetByID(ctx context.Context, id string) (*models.SignJob, error) {
	const q = `
		SELECT id, user_id, input_file_id, signature_file_id,
		       pages, x, y, width, output_file_id, status, created_at
		FROM sign_jobs
		WHERE id = $1
	`
	var (
		job    models.SignJob
		userID sql.NullString
		outID  sql.NullString
	)
	err := r.db.QueryRow(ctx, q, id).Scan(
		&job.ID, &userID, &job.InputFileID, &job.SignatureFileID,
		&job.Pages, &job.X, &job.Y, &job.Width,
		&outID, &job.Status, &job.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if userID.Valid {
		job.UserID = &userID.String
	}
	if outID.Valid {
		job.OutputFileID = &outID.String
	}
	return &job, nil
}

func (r *signRepo) Update(ctx context.Context, job *models.SignJob) error {
	const q = `
		UPDATE sign_jobs SET output_file_id = $1, status = $2 WHERE id = $3
	`
	var out sql.NullString
	if job.OutputFileID != nil {
		out = sql.NullString{String: *job.OutputFileID, Valid: true}
	}
	res, err := r.db.Exec(ctx, q, out, job.Status, job.ID)
	if err != nil {
		r.log.Error("sign: update", logger.Error(err))
		return err
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("sign: no rows for %s", job.ID)
	}
	return nil
}
