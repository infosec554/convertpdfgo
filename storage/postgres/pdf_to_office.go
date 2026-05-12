// PDF→{HTML,PPT,Excel} storage repos. All three have the same shape as
// pdf_to_word_jobs (input_file_id, output_file_id, status, created_at),
// so the CRUD pattern is identical except for the table name. We keep
// the repos separate so each interface keeps its own concrete type and
// storage.New<Foo>Repo signature stays consistent with everything else.
package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/storage"
)

// --- PDF → HTML ----------------------------------------------------------

type pdfToHTMLRepo struct {
	db  *pgxpool.Pool
	log logger.ILogger
}

func NewPDFToHTMLRepo(db *pgxpool.Pool, log logger.ILogger) storage.IPDFToHTMLStorage {
	return &pdfToHTMLRepo{db: db, log: log}
}

func (r *pdfToHTMLRepo) Create(ctx context.Context, job *models.PDFToHTMLJob) error {
	const q = `INSERT INTO pdf_to_html_jobs (id, user_id, input_file_id, output_file_id, status, created_at) VALUES ($1,$2,$3,$4,$5,$6)`
	_, err := r.db.Exec(ctx, q, job.ID, nullableUser(job.UserID), job.InputFileID, nullableID(job.OutputFileID), job.Status, job.CreatedAt)
	if err != nil {
		return fmt.Errorf("pdfToHTMLRepo.Create: %w", err)
	}
	return nil
}

func (r *pdfToHTMLRepo) GetByID(ctx context.Context, id string) (*models.PDFToHTMLJob, error) {
	const q = `SELECT id, user_id, input_file_id, output_file_id, status, created_at FROM pdf_to_html_jobs WHERE id=$1`
	var job models.PDFToHTMLJob
	if err := r.db.QueryRow(ctx, q, id).Scan(&job.ID, &job.UserID, &job.InputFileID, &job.OutputFileID, &job.Status, &job.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("pdfToHTMLRepo.GetByID: not found")
		}
		return nil, fmt.Errorf("pdfToHTMLRepo.GetByID: %w", err)
	}
	return &job, nil
}

func (r *pdfToHTMLRepo) Update(ctx context.Context, job *models.PDFToHTMLJob) error {
	const q = `UPDATE pdf_to_html_jobs SET output_file_id=$1, status=$2 WHERE id=$3`
	_, err := r.db.Exec(ctx, q, job.OutputFileID, job.Status, job.ID)
	if err != nil {
		return fmt.Errorf("pdfToHTMLRepo.Update: %w", err)
	}
	return nil
}

// --- PDF → PPT -----------------------------------------------------------

type pdfToPPTRepo struct {
	db  *pgxpool.Pool
	log logger.ILogger
}

func NewPDFToPPTRepo(db *pgxpool.Pool, log logger.ILogger) storage.IPDFToPPTStorage {
	return &pdfToPPTRepo{db: db, log: log}
}

func (r *pdfToPPTRepo) Create(ctx context.Context, job *models.PDFToPPTJob) error {
	const q = `INSERT INTO pdf_to_ppt_jobs (id, user_id, input_file_id, output_file_id, status, created_at) VALUES ($1,$2,$3,$4,$5,$6)`
	_, err := r.db.Exec(ctx, q, job.ID, nullableUser(job.UserID), job.InputFileID, nullableID(job.OutputFileID), job.Status, job.CreatedAt)
	if err != nil {
		return fmt.Errorf("pdfToPPTRepo.Create: %w", err)
	}
	return nil
}

func (r *pdfToPPTRepo) GetByID(ctx context.Context, id string) (*models.PDFToPPTJob, error) {
	const q = `SELECT id, user_id, input_file_id, output_file_id, status, created_at FROM pdf_to_ppt_jobs WHERE id=$1`
	var job models.PDFToPPTJob
	if err := r.db.QueryRow(ctx, q, id).Scan(&job.ID, &job.UserID, &job.InputFileID, &job.OutputFileID, &job.Status, &job.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("pdfToPPTRepo.GetByID: not found")
		}
		return nil, fmt.Errorf("pdfToPPTRepo.GetByID: %w", err)
	}
	return &job, nil
}

func (r *pdfToPPTRepo) Update(ctx context.Context, job *models.PDFToPPTJob) error {
	const q = `UPDATE pdf_to_ppt_jobs SET output_file_id=$1, status=$2 WHERE id=$3`
	_, err := r.db.Exec(ctx, q, job.OutputFileID, job.Status, job.ID)
	if err != nil {
		return fmt.Errorf("pdfToPPTRepo.Update: %w", err)
	}
	return nil
}

// --- PDF → Excel ---------------------------------------------------------

type pdfToExcelRepo struct {
	db  *pgxpool.Pool
	log logger.ILogger
}

func NewPDFToExcelRepo(db *pgxpool.Pool, log logger.ILogger) storage.IPDFToExcelStorage {
	return &pdfToExcelRepo{db: db, log: log}
}

func (r *pdfToExcelRepo) Create(ctx context.Context, job *models.PDFToExcelJob) error {
	const q = `INSERT INTO pdf_to_excel_jobs (id, user_id, input_file_id, output_file_id, status, created_at) VALUES ($1,$2,$3,$4,$5,$6)`
	_, err := r.db.Exec(ctx, q, job.ID, nullableUser(job.UserID), job.InputFileID, nullableID(job.OutputFileID), job.Status, job.CreatedAt)
	if err != nil {
		return fmt.Errorf("pdfToExcelRepo.Create: %w", err)
	}
	return nil
}

func (r *pdfToExcelRepo) GetByID(ctx context.Context, id string) (*models.PDFToExcelJob, error) {
	const q = `SELECT id, user_id, input_file_id, output_file_id, status, created_at FROM pdf_to_excel_jobs WHERE id=$1`
	var job models.PDFToExcelJob
	if err := r.db.QueryRow(ctx, q, id).Scan(&job.ID, &job.UserID, &job.InputFileID, &job.OutputFileID, &job.Status, &job.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("pdfToExcelRepo.GetByID: not found")
		}
		return nil, fmt.Errorf("pdfToExcelRepo.GetByID: %w", err)
	}
	return &job, nil
}

func (r *pdfToExcelRepo) Update(ctx context.Context, job *models.PDFToExcelJob) error {
	const q = `UPDATE pdf_to_excel_jobs SET output_file_id=$1, status=$2 WHERE id=$3`
	_, err := r.db.Exec(ctx, q, job.OutputFileID, job.Status, job.ID)
	if err != nil {
		return fmt.Errorf("pdfToExcelRepo.Update: %w", err)
	}
	return nil
}

// --- shared helpers ------------------------------------------------------

// nullableUser returns nil when the *string is unset or empty so the
// FK accepts NULL for guest jobs. Pulled out so the three repos above
// stay short.
func nullableUser(u *string) interface{} {
	if u != nil && *u != "" {
		return *u
	}
	return nil
}

func nullableID(p *string) sql.NullString {
	if p != nil {
		return sql.NullString{String: *p, Valid: true}
	}
	return sql.NullString{Valid: false}
}
