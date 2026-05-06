package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/storage"
)

type logRepo struct {
	db  *pgxpool.Pool
	log logger.ILogger
}

// GetLogsByJobID implements storage.ILogService.

func NewLogRepo(db *pgxpool.Pool, log logger.ILogger) storage.ILogService {
	return &logRepo{
		db:  db,
		log: log}
}

func (r *logRepo) GetLogsByJobID(ctx context.Context, jobID string) ([]models.Log, error) {
	query := `
		SELECT id, job_id, job_type, message, level, created_at
		FROM logs
		WHERE job_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, query, jobID)
	if err != nil {
		r.log.Error("failed to query logs by job_id", logger.Error(err))
		return nil, err
	}
	defer rows.Close()

	var logs []models.Log
	for rows.Next() {
		var l models.Log
		err = rows.Scan(
			&l.ID,
			&l.JobID,
			&l.JobType,
			&l.Message,
			&l.Level,
			&l.CreatedAt,
		)
		if err != nil {
			r.log.Error("failed to scan log row", logger.Error(err))
			continue
		}
		logs = append(logs, l)
	}

	if rows.Err() != nil {
		r.log.Error("error iterating log rows", logger.Error(rows.Err()))
		return nil, rows.Err()
	}

	return logs, nil
}
