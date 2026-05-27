package postgres

import (
	"context"
	"database/sql"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
)

type jpgToPDFRepo struct {
	db  *pgxpool.Pool
	log logger.ILogger
}

func NewJPGToPDFRepo(db *pgxpool.Pool, log logger.ILogger) *jpgToPDFRepo {
	return &jpgToPDFRepo{db: db, log: log}
}

// 🧩 PostgreSQL uuid[] formatiga o'tkazish: '{uuid1,uuid2,uuid3}'
func (r *jpgToPDFRepo) formatUUIDArray(ids []string) string {
	return "{" + strings.Join(ids, ",") + "}"
}

// ✅ CREATE FUNCTION
func (r *jpgToPDFRepo) Create(ctx context.Context, job *models.JPGToPDFJob) error {
	// If userID exists, use it, otherwise set it to NULL
	var userID interface{}
	if job.UserID != nil && *job.UserID != "" {
		userID = *job.UserID
	} else {
		userID = nil // NULL for guest users
	}

	// Check for nil OutputFileID and use sql.NullString accordingly
	var outputFileID sql.NullString
	if job.OutputFileID != nil {
		outputFileID = sql.NullString{String: *job.OutputFileID, Valid: true}
	} else {
		outputFileID = sql.NullString{Valid: false} // NULL value in DB
	}

	_, err := r.db.Exec(ctx, `
        INSERT INTO jpg_to_pdf_jobs (
            id, user_id, input_file_ids, output_file_id, status, created_at
        ) VALUES ($1, $2, $3::uuid[], $4, $5, NOW())
    `, job.ID, userID, r.formatUUIDArray(job.InputFileIDs),
		outputFileID, job.Status)

	if err != nil {
		r.log.Error("jpgToPDFRepo.Create failed", logger.Error(err))
	}
	return err
}

// ✅ GET BY ID
func (r *jpgToPDFRepo) GetByID(ctx context.Context, id string) (*models.JPGToPDFJob, error) {
	query := `
		SELECT id, user_id, input_file_ids, output_file_id, status, created_at
		FROM jpg_to_pdf_jobs
		WHERE id = $1
	`

	var job models.JPGToPDFJob
	var inputFileIDs []uuid.UUID
	var outputFileID sql.NullString

	err := r.db.QueryRow(ctx, query, id).Scan(
		&job.ID, &job.UserID, &inputFileIDs,
		&outputFileID, &job.Status, &job.CreatedAt,
	)
	if err != nil {
		r.log.Error("jpgToPDFRepo.GetByID failed", logger.Error(err))
		return nil, err
	}

	// uuid.UUID [] ni string ga aylantiramiz, agar kerak bo‘lsa
	for _, uuidVal := range inputFileIDs {
		job.InputFileIDs = append(job.InputFileIDs, uuidVal.String())
	}

	if outputFileID.Valid {
		job.OutputFileID = &outputFileID.String
	}

	return &job, nil
}

// 🧩 Postgres'dan kelgan '{uuid1,uuid2}' stringni []string ga aylantirish
func (r *jpgToPDFRepo) parseUUIDArray(input string) []string {
	input = strings.Trim(input, "{}") // remove outer curly braces
	if input == "" {
		return []string{}
	}
	return strings.Split(input, ",")
}

// ✅ UPDATE status + output
// outputFileID may be "" when marking a job as "failed" — convert to SQL NULL
// so postgres doesn't reject it against the UUID column type.
func (r *jpgToPDFRepo) UpdateStatusAndOutput(ctx context.Context, id, status, outputFileID string) error {
	var outID any
	if outputFileID == "" {
		outID = nil
	} else {
		outID = outputFileID
	}

	query := `
		UPDATE jpg_to_pdf_jobs
		SET status = $1, output_file_id = $2
		WHERE id = $3
	`

	_, err := r.db.Exec(ctx, query, status, outID, id)
	if err != nil {
		r.log.Error("jpgToPDFRepo.UpdateStatusAndOutput failed", logger.Error(err))
	}
	return err
}
