package postgres

import (
	"context"
	"database/sql"

	"github.com/jackc/pgx/v5/pgxpool"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
)

type pptxBuilderRepo struct {
	db  *pgxpool.Pool
	log logger.ILogger
}

func NewPPTXBuilderRepo(db *pgxpool.Pool, log logger.ILogger) *pptxBuilderRepo {
	return &pptxBuilderRepo{db: db, log: log}
}

func (r *pptxBuilderRepo) Create(ctx context.Context, j *models.PPTXBuilderJob) error {
	const q = `
		INSERT INTO pptx_builder_jobs
		    (id, user_id, topic, slide_count, tone, language, audience, status, error, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
	`
	_, err := r.db.Exec(ctx, q,
		j.ID, handleUserID(j.UserID), j.Topic, j.SlideCount, j.Tone, j.Language, j.Audience,
		j.Status, j.Error, j.CreatedAt, j.UpdatedAt,
	)
	if err != nil {
		r.log.Error("pptx_builder: create", logger.Error(err))
	}
	return err
}

func (r *pptxBuilderRepo) GetByID(ctx context.Context, id string) (*models.PPTXBuilderJob, error) {
	const q = `
		SELECT id, user_id, topic, slide_count, tone, language, audience,
		       status, error, output_file_id, created_at, updated_at
		FROM pptx_builder_jobs WHERE id = $1
	`
	var (
		j      models.PPTXBuilderJob
		userID sql.NullString
		outID  sql.NullString
	)
	err := r.db.QueryRow(ctx, q, id).Scan(
		&j.ID, &userID, &j.Topic, &j.SlideCount, &j.Tone, &j.Language, &j.Audience,
		&j.Status, &j.Error, &outID, &j.CreatedAt, &j.UpdatedAt,
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

func (r *pptxBuilderRepo) Update(ctx context.Context, j *models.PPTXBuilderJob) error {
	const q = `
		UPDATE pptx_builder_jobs
		   SET status = $2, error = $3, output_file_id = $4, updated_at = NOW()
		 WHERE id = $1
	`
	var outID interface{}
	if j.OutputFileID != nil {
		outID = *j.OutputFileID
	}
	_, err := r.db.Exec(ctx, q, j.ID, j.Status, j.Error, outID)
	if err != nil {
		r.log.Error("pptx_builder: update", logger.Error(err))
	}
	return err
}
