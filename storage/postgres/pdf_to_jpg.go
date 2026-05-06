package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/storage"
)

type pdfToJPGRepo struct {
	db  *pgxpool.Pool
	log logger.ILogger
}

func NewPDFToJPGRepo(db *pgxpool.Pool, log logger.ILogger) storage.IPDFToJPGStorage {
	return &pdfToJPGRepo{db: db, log: log}
}

func (r *pdfToJPGRepo) Create(ctx context.Context, job *models.PDFToJPGJob) error {
	var userID interface{}
	if job.UserID != nil {
		userID = *job.UserID // Foydalanuvchi ID mavjud bo'lsa, saqlaymiz
	} else {
		userID = nil // NULL bo'lishi mumkin
	}

	_, err := r.db.Exec(ctx, `
		INSERT INTO pdf_to_jpg_jobs 
		(id, user_id, input_file_id, output_file_ids, zip_file_id, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, job.ID, userID, job.InputFileID, job.OutputFileIDs, job.ZipFileID, job.Status, job.CreatedAt)

	return err
}
func (r *pdfToJPGRepo) GetByID(ctx context.Context, id string) (*models.PDFToJPGJob, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, user_id, input_file_id, output_file_ids, zip_file_id, status, created_at
		FROM pdf_to_jpg_jobs WHERE id = $1
	`, id)

	var job models.PDFToJPGJob
	err := row.Scan(
		&job.ID, &job.UserID, &job.InputFileID,
		&job.OutputFileIDs, &job.ZipFileID,
		&job.Status, &job.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func (r *pdfToJPGRepo) Update(ctx context.Context, job *models.PDFToJPGJob) error {
	_, err := r.db.Exec(ctx, `
		UPDATE pdf_to_jpg_jobs 
		SET output_file_ids=$1, zip_file_id=$2, status=$3 
		WHERE id=$4
	`, job.OutputFileIDs, job.ZipFileID, job.Status, job.ID)
	return err
}
