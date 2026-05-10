package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/storage"
)

type splitRepo struct {
	db  *pgxpool.Pool
	log logger.ILogger
}

func NewSplitRepo(db *pgxpool.Pool, log logger.ILogger) storage.ISplitStorage {
	return &splitRepo{
		db:  db,
		log: log,
	}
}

func (r *splitRepo) Create(ctx context.Context, job *models.SplitJob) error {
	query := `
		INSERT INTO split_jobs (
			id,
			user_id,
			input_file_id,
			split_ranges,
			output_file_ids,
			status,
			created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	var userID interface{}
	if job.UserID != nil && *job.UserID != "" {
		userID = *job.UserID
	} else {
		userID = nil // NULL bo'ladi agar guest bo'lsa
	}

	_, err := r.db.Exec(ctx, query,
		job.ID,
		userID,
		job.InputFileID,
		job.SplitRanges,
		job.OutputFileIDs,
		job.Status,
		job.CreatedAt,
	)

	if err != nil {
		r.log.Error("Failed to insert split job", logger.Any("error", err))
	}
	return err
}

// output_file_ids is a UUID[] in postgres, not jsonb — scan it as a text[]
// (cast on read) so pgx-v5 can decode it into a []string.
func (r *splitRepo) GetByID(ctx context.Context, id string) (*models.SplitJob, error) {
	var job models.SplitJob

	const query = `
		SELECT id, user_id, input_file_id, split_ranges,
		       COALESCE(output_file_ids, ARRAY[]::uuid[])::text[],
		       status, created_at
		FROM split_jobs
		WHERE id = $1
	`

	err := r.db.QueryRow(ctx, query, id).Scan(
		&job.ID,
		&job.UserID,
		&job.InputFileID,
		&job.SplitRanges,
		&job.OutputFileIDs,
		&job.Status,
		&job.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &job, nil
}

// UpdateOutputFiles bumps status='done' atomically with the IDs — the
// service treats successful upload of every output as completion. Without
// this, status was stuck on "pending" forever even after a successful split.
func (r *splitRepo) UpdateOutputFiles(ctx context.Context, jobID string, outputIDs []string) error {
	query := `
		UPDATE split_jobs
		SET output_file_ids = $1, status = 'done'
		WHERE id = $2
	`
	_, err := r.db.Exec(ctx, query, outputIDs, jobID)
	return err
}
