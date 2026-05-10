package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
)

type nupRepo struct {
	db  *pgxpool.Pool
	log logger.ILogger
}

func NewNUpRepo(db *pgxpool.Pool, log logger.ILogger) *nupRepo {
	return &nupRepo{db: db, log: log}
}

func (r *nupRepo) Create(ctx context.Context, job *models.NUpJob) error {
	const q = `
		INSERT INTO nup_jobs
		    (id, user_id, input_file_id, n, orientation, border, status, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
	`
	_, err := r.db.Exec(ctx, q, job.ID, handleUserID(job.UserID), job.InputFileID,
		job.N, job.Orientation, job.Border, job.Status, job.CreatedAt)
	if err != nil {
		r.log.Error("nup: create", logger.Error(err))
	}
	return err
}

func (r *nupRepo) GetByID(ctx context.Context, id string) (*models.NUpJob, error) {
	const q = `
		SELECT id, user_id, input_file_id, n, orientation, border,
		       output_file_id, status, created_at
		FROM nup_jobs WHERE id = $1
	`
	var (
		job        models.NUpJob
		userID     sql.NullString
		outputFile sql.NullString
	)
	err := r.db.QueryRow(ctx, q, id).Scan(&job.ID, &userID, &job.InputFileID,
		&job.N, &job.Orientation, &job.Border, &outputFile, &job.Status, &job.CreatedAt)
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

func (r *nupRepo) Update(ctx context.Context, job *models.NUpJob) error {
	const q = `UPDATE nup_jobs SET output_file_id=$1, status=$2 WHERE id=$3`
	var out sql.NullString
	if job.OutputFileID != nil {
		out = sql.NullString{String: *job.OutputFileID, Valid: true}
	}
	res, err := r.db.Exec(ctx, q, out, job.Status, job.ID)
	if err != nil {
		r.log.Error("nup: update", logger.Error(err))
		return err
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("nup: no rows for %s", job.ID)
	}
	return nil
}
