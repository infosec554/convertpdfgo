package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/storage"
)

type mergeRepo struct {
	db  *pgxpool.Pool
	log logger.ILogger
}

func NewMergeRepo(db *pgxpool.Pool, log logger.ILogger) storage.IMergeStorage {
	return &mergeRepo{
		db:  db,
		log: log,
	}
}

// ✅ MergeJob yaratish
func (m *mergeRepo) Create(ctx context.Context, job *models.MergeJob) error {
	query := `
		INSERT INTO merge_jobs (id, user_id, output_file_id, status, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`

	var userID interface{}
	if job.UserID != nil && *job.UserID != "" {
		userID = *job.UserID
	} else {
		userID = nil // guest bo‘lishi mumkin
	}

	var outputFileID interface{}
	if job.OutputFileID != nil {
		outputFileID = *job.OutputFileID
	} else {
		outputFileID = nil
	}

	_, err := m.db.Exec(ctx, query,
		job.ID, userID, outputFileID, job.Status, job.CreatedAt)
	if err != nil {
		m.log.Error("❌ failed to create merge job", logger.Error(err))
	}
	return err
}

// ✅ MergeJob ma'lumotini olish
func (m *mergeRepo) GetByID(ctx context.Context, id string) (*models.MergeJob, error) {
	query := `
		SELECT id, user_id, output_file_id, status, created_at
		FROM merge_jobs
		WHERE id = $1
	`

	var job models.MergeJob
	err := m.db.QueryRow(ctx, query, id).Scan(
		&job.ID, &job.UserID, &job.OutputFileID, &job.Status, &job.CreatedAt,
	)
	if err != nil {
		m.log.Error("❌ failed to fetch merge job", logger.Error(err))
		return nil, err
	}

	inputs, err := m.GetInputFiles(ctx, job.ID)
	if err != nil {
		m.log.Error("⚠️ failed to fetch input files for merge job", logger.Error(err))
	} else {
		job.InputFileIDs = inputs
	}

	return &job, nil
}

func (m *mergeRepo) AddInputFiles(ctx context.Context, jobID string, fileIDs []string) error {
	query := `
		INSERT INTO merge_job_input_files (id, job_id, file_id)
		VALUES ($1, $2, $3)
	`

	for _, fileID := range fileIDs {
		_, err := m.db.Exec(ctx, query, uuid.New().String(), jobID, fileID)
		if err != nil {
			m.log.Error("❌ failed to add input file", logger.String("fileID", fileID), logger.Error(err))
			return err
		}
	}

	return nil
}

func (m *mergeRepo) GetInputFiles(ctx context.Context, jobID string) ([]string, error) {
	query := `SELECT file_id FROM merge_job_input_files WHERE job_id = $1`
	rows, err := m.db.Query(ctx, query, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fileIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		fileIDs = append(fileIDs, id)
	}
	return fileIDs, nil
}

// ✅ Job holatini yangilash
func (m *mergeRepo) Update(ctx context.Context, job *models.MergeJob) error {
	query := `
		UPDATE merge_jobs
		SET output_file_id = $1, status = $2
		WHERE id = $3
	`

	var outputFileID interface{}
	if job.OutputFileID != nil {
		outputFileID = *job.OutputFileID
	} else {
		outputFileID = nil
	}

	_, err := m.db.Exec(ctx, query, outputFileID, job.Status, job.ID)
	if err != nil {
		m.log.Error("❌ failed to update merge job", logger.Error(err))
	}
	return err
}
