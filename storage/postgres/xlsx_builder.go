package postgres

import (
	"context"
	"database/sql"

	"github.com/jackc/pgx/v5/pgxpool"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
)

type xlsxBuilderRepo struct {
	db  *pgxpool.Pool
	log logger.ILogger
}

func NewXLSXBuilderRepo(db *pgxpool.Pool, log logger.ILogger) *xlsxBuilderRepo {
	return &xlsxBuilderRepo{db: db, log: log}
}

func (r *xlsxBuilderRepo) Create(ctx context.Context, j *models.XLSXBuilderJob) error {
	const q = `
		INSERT INTO xlsx_builder_jobs
		    (id, user_id, topic, sheet_count, tone, language, audience, include_chart,
		     status, error, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
	`
	_, err := r.db.Exec(ctx, q,
		j.ID, handleUserID(j.UserID), j.Topic, j.SheetCount, j.Tone, j.Language, j.Audience,
		j.IncludeChart, j.Status, j.Error, j.CreatedAt, j.UpdatedAt,
	)
	if err != nil {
		r.log.Error("xlsx_builder: create", logger.Error(err))
	}
	return err
}

func (r *xlsxBuilderRepo) GetByID(ctx context.Context, id string) (*models.XLSXBuilderJob, error) {
	const q = `
		SELECT id, user_id, topic, sheet_count, tone, language, audience,
		       include_chart, status, error, output_file_id, created_at, updated_at
		FROM xlsx_builder_jobs WHERE id = $1
	`
	var (
		j      models.XLSXBuilderJob
		userID sql.NullString
		outID  sql.NullString
	)
	err := r.db.QueryRow(ctx, q, id).Scan(
		&j.ID, &userID, &j.Topic, &j.SheetCount, &j.Tone, &j.Language, &j.Audience,
		&j.IncludeChart, &j.Status, &j.Error, &outID, &j.CreatedAt, &j.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if userID.Valid {
		j.UserID = &userID.String
	}
	if outID.Valid {
		j.OutputFileID = &outID.String
	}
	return &j, nil
}

func (r *xlsxBuilderRepo) Update(ctx context.Context, j *models.XLSXBuilderJob) error {
	const q = `
		UPDATE xlsx_builder_jobs
		   SET status = $2, error = $3, output_file_id = $4, updated_at = NOW()
		 WHERE id = $1
	`
	var outID interface{}
	if j.OutputFileID != nil {
		outID = *j.OutputFileID
	}
	_, err := r.db.Exec(ctx, q, j.ID, j.Status, j.Error, outID)
	if err != nil {
		r.log.Error("xlsx_builder: update", logger.Error(err))
	}
	return err
}
