package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
)

type metadataRepo struct {
	db  *pgxpool.Pool
	log logger.ILogger
}

func NewMetadataRepo(db *pgxpool.Pool, log logger.ILogger) *metadataRepo {
	return &metadataRepo{db: db, log: log}
}

func nullable(s *string) interface{} {
	if s == nil {
		return nil
	}
	return *s
}

func (r *metadataRepo) Create(ctx context.Context, job *models.MetadataJob) error {
	const q = `
		INSERT INTO metadata_jobs
		    (id, user_id, input_file_id, title, author, subject, keywords,
		     creator, status, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
	`
	_, err := r.db.Exec(ctx, q,
		job.ID, handleUserID(job.UserID), job.InputFileID,
		nullable(job.Title), nullable(job.Author), nullable(job.Subject),
		nullable(job.Keywords), nullable(job.Creator),
		job.Status, job.CreatedAt,
	)
	if err != nil {
		r.log.Error("metadata: create", logger.Error(err))
	}
	return err
}

func (r *metadataRepo) GetByID(ctx context.Context, id string) (*models.MetadataJob, error) {
	const q = `
		SELECT id, user_id, input_file_id, title, author, subject, keywords,
		       creator, output_file_id, status, created_at
		FROM metadata_jobs WHERE id = $1
	`
	var (
		job                                              models.MetadataJob
		userID                                           sql.NullString
		title, author, subject, keywords, creator, outID sql.NullString
	)
	err := r.db.QueryRow(ctx, q, id).Scan(
		&job.ID, &userID, &job.InputFileID,
		&title, &author, &subject, &keywords, &creator,
		&outID, &job.Status, &job.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if userID.Valid {
		job.UserID = &userID.String
	}
	if title.Valid {
		job.Title = &title.String
	}
	if author.Valid {
		job.Author = &author.String
	}
	if subject.Valid {
		job.Subject = &subject.String
	}
	if keywords.Valid {
		job.Keywords = &keywords.String
	}
	if creator.Valid {
		job.Creator = &creator.String
	}
	if outID.Valid {
		job.OutputFileID = &outID.String
	}
	return &job, nil
}

func (r *metadataRepo) Update(ctx context.Context, job *models.MetadataJob) error {
	const q = `UPDATE metadata_jobs SET output_file_id=$1, status=$2 WHERE id=$3`
	var out sql.NullString
	if job.OutputFileID != nil {
		out = sql.NullString{String: *job.OutputFileID, Valid: true}
	}
	res, err := r.db.Exec(ctx, q, out, job.Status, job.ID)
	if err != nil {
		r.log.Error("metadata: update", logger.Error(err))
		return err
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("metadata: no rows for %s", job.ID)
	}
	return nil
}
