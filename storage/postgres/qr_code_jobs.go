package postgres

import (
	"context"
	"database/sql"

	"github.com/jackc/pgx/v5/pgxpool"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/storage"
)

type qrCodeRepo struct {
	db  *pgxpool.Pool
	log logger.ILogger
}

func NewQRCodeRepo(db *pgxpool.Pool, log logger.ILogger) storage.IQRCodeStorage {
	return &qrCodeRepo{
		db:  db,
		log: log,
	}
}

func (r *qrCodeRepo) Create(ctx context.Context, job *models.QRCodeJob) error {
	query := `
		INSERT INTO qr_code_jobs (id, user_id, input_file_id, qr_content, position, size, status, created_at)
		VALUES ($1, NULLIF($2,'')::uuid, $3, $4, $5, $6, $7, $8)
	`
	_, err := r.db.Exec(ctx, query,
		job.ID, job.UserID, job.InputFileID, job.QRContent, job.Position, job.Size, job.Status, job.CreatedAt)
	return err
}

func (r *qrCodeRepo) Update(ctx context.Context, job *models.QRCodeJob) error {
	// OutputFileID is a plain string in the model — empty on markFailed
	// requires NULLIF before binding to the UUID column.
	query := `
		UPDATE qr_code_jobs
		SET output_file_id = NULLIF($1,'')::uuid, status = $2
		WHERE id = $3
	`
	_, err := r.db.Exec(ctx, query, job.OutputFileID, job.Status, job.ID)
	return err
}

func (r *qrCodeRepo) GetByID(ctx context.Context, id string) (*models.QRCodeJob, error) {
	query := `
		SELECT id, COALESCE(user_id::text,''), input_file_id, qr_content, position, size, output_file_id, status, created_at
		FROM qr_code_jobs
		WHERE id = $1
	`
	var (
		job          models.QRCodeJob
		outputFileID sql.NullString
	)
	err := r.db.QueryRow(ctx, query, id).Scan(
		&job.ID, &job.UserID, &job.InputFileID, &job.QRContent, &job.Position, &job.Size,
		&outputFileID, &job.Status, &job.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if outputFileID.Valid {
		job.OutputFileID = &outputFileID.String
	}
	return &job, nil
}
