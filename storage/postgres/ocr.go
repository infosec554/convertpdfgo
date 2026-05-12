// OCR job repos. Five of the six follow the now-familiar
// (input_file, output_file, lang, status) shape and share two helpers
// from pdf_to_office.go (nullableUser / nullableID). ScanToPDF stores
// an array of input file IDs; OCRDetectLang has no produced file and
// stuffs the result on the row instead.
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

// --- /api/pdf/ocr ----------------------------------------------------------

type ocrRepo struct {
	db  *pgxpool.Pool
	log logger.ILogger
}

func NewOCRRepo(db *pgxpool.Pool, log logger.ILogger) storage.IOCRStorage {
	return &ocrRepo{db: db, log: log}
}

func (r *ocrRepo) Create(ctx context.Context, job *models.OCRJob) error {
	const q = `INSERT INTO ocr_jobs (id, user_id, input_file_id, output_file_id, lang, force_ocr, status, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`
	_, err := r.db.Exec(ctx, q, job.ID, nullableUser(job.UserID), job.InputFileID, nullableID(job.OutputFileID), job.Lang, job.ForceOCR, job.Status, job.CreatedAt)
	if err != nil {
		return fmt.Errorf("ocrRepo.Create: %w", err)
	}
	return nil
}

func (r *ocrRepo) GetByID(ctx context.Context, id string) (*models.OCRJob, error) {
	const q = `SELECT id, user_id, input_file_id, output_file_id, lang, force_ocr, status, created_at FROM ocr_jobs WHERE id=$1`
	var j models.OCRJob
	if err := r.db.QueryRow(ctx, q, id).Scan(&j.ID, &j.UserID, &j.InputFileID, &j.OutputFileID, &j.Lang, &j.ForceOCR, &j.Status, &j.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("ocrRepo.GetByID: not found")
		}
		return nil, fmt.Errorf("ocrRepo.GetByID: %w", err)
	}
	return &j, nil
}

func (r *ocrRepo) Update(ctx context.Context, job *models.OCRJob) error {
	const q = `UPDATE ocr_jobs SET output_file_id=$1, status=$2 WHERE id=$3`
	_, err := r.db.Exec(ctx, q, job.OutputFileID, job.Status, job.ID)
	if err != nil {
		return fmt.Errorf("ocrRepo.Update: %w", err)
	}
	return nil
}

// --- /api/pdf/scan-to-pdf --------------------------------------------------

type scanToPDFRepo struct {
	db  *pgxpool.Pool
	log logger.ILogger
}

func NewScanToPDFRepo(db *pgxpool.Pool, log logger.ILogger) storage.IScanToPDFStorage {
	return &scanToPDFRepo{db: db, log: log}
}

func (r *scanToPDFRepo) Create(ctx context.Context, job *models.ScanToPDFJob) error {
	const q = `INSERT INTO scan_to_pdf_jobs (id, user_id, input_file_ids, output_file_id, lang, status, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7)`
	_, err := r.db.Exec(ctx, q, job.ID, nullableUser(job.UserID), job.InputFileIDs, nullableID(job.OutputFileID), job.Lang, job.Status, job.CreatedAt)
	if err != nil {
		return fmt.Errorf("scanToPDFRepo.Create: %w", err)
	}
	return nil
}

func (r *scanToPDFRepo) GetByID(ctx context.Context, id string) (*models.ScanToPDFJob, error) {
	const q = `SELECT id, user_id, input_file_ids, output_file_id, lang, status, created_at FROM scan_to_pdf_jobs WHERE id=$1`
	var j models.ScanToPDFJob
	if err := r.db.QueryRow(ctx, q, id).Scan(&j.ID, &j.UserID, &j.InputFileIDs, &j.OutputFileID, &j.Lang, &j.Status, &j.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("scanToPDFRepo.GetByID: not found")
		}
		return nil, fmt.Errorf("scanToPDFRepo.GetByID: %w", err)
	}
	return &j, nil
}

func (r *scanToPDFRepo) Update(ctx context.Context, job *models.ScanToPDFJob) error {
	const q = `UPDATE scan_to_pdf_jobs SET output_file_id=$1, status=$2 WHERE id=$3`
	_, err := r.db.Exec(ctx, q, job.OutputFileID, job.Status, job.ID)
	if err != nil {
		return fmt.Errorf("scanToPDFRepo.Update: %w", err)
	}
	return nil
}

// --- /api/pdf/ocr-extract-text --------------------------------------------

type ocrExtractTextRepo struct {
	db  *pgxpool.Pool
	log logger.ILogger
}

func NewOCRExtractTextRepo(db *pgxpool.Pool, log logger.ILogger) storage.IOCRExtractTextStorage {
	return &ocrExtractTextRepo{db: db, log: log}
}

func (r *ocrExtractTextRepo) Create(ctx context.Context, job *models.OCRExtractTextJob) error {
	const q = `INSERT INTO ocr_extract_text_jobs (id, user_id, input_file_id, output_file_id, lang, status, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7)`
	_, err := r.db.Exec(ctx, q, job.ID, nullableUser(job.UserID), job.InputFileID, nullableID(job.OutputFileID), job.Lang, job.Status, job.CreatedAt)
	if err != nil {
		return fmt.Errorf("ocrExtractTextRepo.Create: %w", err)
	}
	return nil
}

func (r *ocrExtractTextRepo) GetByID(ctx context.Context, id string) (*models.OCRExtractTextJob, error) {
	const q = `SELECT id, user_id, input_file_id, output_file_id, lang, status, created_at FROM ocr_extract_text_jobs WHERE id=$1`
	var j models.OCRExtractTextJob
	if err := r.db.QueryRow(ctx, q, id).Scan(&j.ID, &j.UserID, &j.InputFileID, &j.OutputFileID, &j.Lang, &j.Status, &j.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("ocrExtractTextRepo.GetByID: not found")
		}
		return nil, fmt.Errorf("ocrExtractTextRepo.GetByID: %w", err)
	}
	return &j, nil
}

func (r *ocrExtractTextRepo) Update(ctx context.Context, job *models.OCRExtractTextJob) error {
	const q = `UPDATE ocr_extract_text_jobs SET output_file_id=$1, status=$2 WHERE id=$3`
	_, err := r.db.Exec(ctx, q, job.OutputFileID, job.Status, job.ID)
	if err != nil {
		return fmt.Errorf("ocrExtractTextRepo.Update: %w", err)
	}
	return nil
}

// --- /api/pdf/image-to-text -----------------------------------------------

type imageToTextRepo struct {
	db  *pgxpool.Pool
	log logger.ILogger
}

func NewImageToTextRepo(db *pgxpool.Pool, log logger.ILogger) storage.IImageToTextStorage {
	return &imageToTextRepo{db: db, log: log}
}

func (r *imageToTextRepo) Create(ctx context.Context, job *models.ImageToTextJob) error {
	const q = `INSERT INTO image_to_text_jobs (id, user_id, input_file_id, output_file_id, lang, status, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7)`
	_, err := r.db.Exec(ctx, q, job.ID, nullableUser(job.UserID), job.InputFileID, nullableID(job.OutputFileID), job.Lang, job.Status, job.CreatedAt)
	if err != nil {
		return fmt.Errorf("imageToTextRepo.Create: %w", err)
	}
	return nil
}

func (r *imageToTextRepo) GetByID(ctx context.Context, id string) (*models.ImageToTextJob, error) {
	const q = `SELECT id, user_id, input_file_id, output_file_id, lang, status, created_at FROM image_to_text_jobs WHERE id=$1`
	var j models.ImageToTextJob
	if err := r.db.QueryRow(ctx, q, id).Scan(&j.ID, &j.UserID, &j.InputFileID, &j.OutputFileID, &j.Lang, &j.Status, &j.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("imageToTextRepo.GetByID: not found")
		}
		return nil, fmt.Errorf("imageToTextRepo.GetByID: %w", err)
	}
	return &j, nil
}

func (r *imageToTextRepo) Update(ctx context.Context, job *models.ImageToTextJob) error {
	const q = `UPDATE image_to_text_jobs SET output_file_id=$1, status=$2 WHERE id=$3`
	_, err := r.db.Exec(ctx, q, job.OutputFileID, job.Status, job.ID)
	if err != nil {
		return fmt.Errorf("imageToTextRepo.Update: %w", err)
	}
	return nil
}

// --- /api/pdf/ocr-detect-lang ---------------------------------------------

type ocrDetectLangRepo struct {
	db  *pgxpool.Pool
	log logger.ILogger
}

func NewOCRDetectLangRepo(db *pgxpool.Pool, log logger.ILogger) storage.IOCRDetectLangStorage {
	return &ocrDetectLangRepo{db: db, log: log}
}

func (r *ocrDetectLangRepo) Create(ctx context.Context, job *models.OCRDetectLangJob) error {
	const q = `INSERT INTO ocr_detect_lang_jobs (id, user_id, input_file_id, detected_lang, confidence, status, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7)`
	var dl sql.NullString
	if job.DetectedLang != nil {
		dl = sql.NullString{String: *job.DetectedLang, Valid: true}
	}
	var conf sql.NullFloat64
	if job.Confidence != nil {
		conf = sql.NullFloat64{Float64: *job.Confidence, Valid: true}
	}
	_, err := r.db.Exec(ctx, q, job.ID, nullableUser(job.UserID), job.InputFileID, dl, conf, job.Status, job.CreatedAt)
	if err != nil {
		return fmt.Errorf("ocrDetectLangRepo.Create: %w", err)
	}
	return nil
}

func (r *ocrDetectLangRepo) GetByID(ctx context.Context, id string) (*models.OCRDetectLangJob, error) {
	const q = `SELECT id, user_id, input_file_id, detected_lang, confidence, status, created_at FROM ocr_detect_lang_jobs WHERE id=$1`
	var j models.OCRDetectLangJob
	var dl sql.NullString
	var conf sql.NullFloat64
	if err := r.db.QueryRow(ctx, q, id).Scan(&j.ID, &j.UserID, &j.InputFileID, &dl, &conf, &j.Status, &j.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("ocrDetectLangRepo.GetByID: not found")
		}
		return nil, fmt.Errorf("ocrDetectLangRepo.GetByID: %w", err)
	}
	if dl.Valid {
		j.DetectedLang = &dl.String
	}
	if conf.Valid {
		v := conf.Float64
		j.Confidence = &v
	}
	return &j, nil
}

func (r *ocrDetectLangRepo) Update(ctx context.Context, job *models.OCRDetectLangJob) error {
	const q = `UPDATE ocr_detect_lang_jobs SET detected_lang=$1, confidence=$2, status=$3 WHERE id=$4`
	var dl sql.NullString
	if job.DetectedLang != nil {
		dl = sql.NullString{String: *job.DetectedLang, Valid: true}
	}
	var conf sql.NullFloat64
	if job.Confidence != nil {
		conf = sql.NullFloat64{Float64: *job.Confidence, Valid: true}
	}
	_, err := r.db.Exec(ctx, q, dl, conf, job.Status, job.ID)
	if err != nil {
		return fmt.Errorf("ocrDetectLangRepo.Update: %w", err)
	}
	return nil
}

// --- /api/pdf/ocr-structured ----------------------------------------------

type ocrStructuredRepo struct {
	db  *pgxpool.Pool
	log logger.ILogger
}

func NewOCRStructuredRepo(db *pgxpool.Pool, log logger.ILogger) storage.IOCRStructuredStorage {
	return &ocrStructuredRepo{db: db, log: log}
}

func (r *ocrStructuredRepo) Create(ctx context.Context, job *models.OCRStructuredJob) error {
	const q = `INSERT INTO ocr_structured_jobs (id, user_id, input_file_id, output_file_id, lang, status, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7)`
	_, err := r.db.Exec(ctx, q, job.ID, nullableUser(job.UserID), job.InputFileID, nullableID(job.OutputFileID), job.Lang, job.Status, job.CreatedAt)
	if err != nil {
		return fmt.Errorf("ocrStructuredRepo.Create: %w", err)
	}
	return nil
}

func (r *ocrStructuredRepo) GetByID(ctx context.Context, id string) (*models.OCRStructuredJob, error) {
	const q = `SELECT id, user_id, input_file_id, output_file_id, lang, status, created_at FROM ocr_structured_jobs WHERE id=$1`
	var j models.OCRStructuredJob
	if err := r.db.QueryRow(ctx, q, id).Scan(&j.ID, &j.UserID, &j.InputFileID, &j.OutputFileID, &j.Lang, &j.Status, &j.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("ocrStructuredRepo.GetByID: not found")
		}
		return nil, fmt.Errorf("ocrStructuredRepo.GetByID: %w", err)
	}
	return &j, nil
}

func (r *ocrStructuredRepo) Update(ctx context.Context, job *models.OCRStructuredJob) error {
	const q = `UPDATE ocr_structured_jobs SET output_file_id=$1, status=$2 WHERE id=$3`
	_, err := r.db.Exec(ctx, q, job.OutputFileID, job.Status, job.ID)
	if err != nil {
		return fmt.Errorf("ocrStructuredRepo.Update: %w", err)
	}
	return nil
}
