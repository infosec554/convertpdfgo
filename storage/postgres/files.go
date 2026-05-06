package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/storage"
)

type fileRepo struct {
	db  *pgxpool.Pool
	log logger.ILogger
}

func NewFileRepo(db *pgxpool.Pool, log logger.ILogger) storage.IFileStorage {
	return &fileRepo{db: db, log: log}
}

func (f *fileRepo) Save(ctx context.Context, file models.File) (string, error) {
	query := `
		INSERT INTO files (id, user_id, file_name, file_path, file_type, file_size, sha256, uploaded_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := f.db.Exec(ctx, query,
		file.ID, file.UserID, file.FileName, file.FilePath,
		file.FileType, file.FileSize, file.SHA256, file.UploadedAt)
	if err != nil {
		f.log.Error("DB insert error", logger.Error(err))
		return "", err
	}
	return file.ID, nil
}

func (f *fileRepo) GetByID(ctx context.Context, id string) (models.File, error) {
	var file models.File
	query := `
		SELECT id, user_id, file_name, file_path, file_type, file_size, sha256, uploaded_at
		FROM files WHERE id = $1
	`
	err := f.db.QueryRow(ctx, query, id).Scan(
		&file.ID, &file.UserID, &file.FileName, &file.FilePath,
		&file.FileType, &file.FileSize, &file.SHA256, &file.UploadedAt)
	if err != nil {
		f.log.Error("failed to fetch file", logger.Error(err))
		return models.File{}, err
	}
	return file, nil
}

func (f *fileRepo) Delete(ctx context.Context, id string) error {
	_, err := f.db.Exec(ctx, `DELETE FROM files WHERE id = $1`, id)
	if err != nil {
		f.log.Error("failed to delete file", logger.Error(err))
	}
	return err
}

func (f *fileRepo) ListByUser(ctx context.Context, userID string) ([]models.File, error) {
	query := `
		SELECT id, user_id, file_name, file_path, file_type, file_size, sha256, uploaded_at
		FROM files WHERE user_id = $1 ORDER BY uploaded_at DESC
	`
	rows, err := f.db.Query(ctx, query, userID)
	if err != nil {
		f.log.Error("failed to fetch user files", logger.Error(err))
		return nil, err
	}
	defer rows.Close()

	var files []models.File
	for rows.Next() {
		var file models.File
		if err := rows.Scan(&file.ID, &file.UserID, &file.FileName, &file.FilePath,
			&file.FileType, &file.FileSize, &file.SHA256, &file.UploadedAt); err != nil {
			f.log.Error("error scanning row", logger.Error(err))
			continue
		}
		files = append(files, file)
	}
	return files, nil
}

func (f *fileRepo) FindBySHA256(ctx context.Context, sha string) (models.File, error) {
	var file models.File
	query := `
		SELECT id, user_id, file_name, file_path, file_type, file_size, sha256, uploaded_at
		FROM files
		WHERE sha256 = $1
		ORDER BY uploaded_at DESC
		LIMIT 1
	`
	err := f.db.QueryRow(ctx, query, sha).Scan(
		&file.ID, &file.UserID, &file.FileName, &file.FilePath,
		&file.FileType, &file.FileSize, &file.SHA256, &file.UploadedAt)
	if err != nil {
		return models.File{}, err
	}
	return file, nil
}

func (f *fileRepo) GetOldFiles(ctx context.Context, olderThanDays int) ([]models.OldFile, error) {
	query := `
		SELECT id, file_path
		FROM files
		WHERE uploaded_at < NOW() - ($1 || ' days')::INTERVAL
	`
	rows, err := f.db.Query(ctx, query, olderThanDays)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var oldFiles []models.OldFile
	for rows.Next() {
		var of models.OldFile
		if err := rows.Scan(&of.ID, &of.FilePath); err != nil {
			continue
		}
		oldFiles = append(oldFiles, of)
	}
	return oldFiles, nil
}

func (f *fileRepo) GetGuestFilesOlderThanMinutes(ctx context.Context, minutes int) ([]models.OldFile, error) {
	query := `
		SELECT id, file_path
		FROM files
		WHERE user_id IS NULL
		  AND uploaded_at < NOW() - ($1 || ' minutes')::INTERVAL
	`
	rows, err := f.db.Query(ctx, query, minutes)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.OldFile
	for rows.Next() {
		var of models.OldFile
		if err := rows.Scan(&of.ID, &of.FilePath); err != nil {
			continue
		}
		out = append(out, of)
	}
	return out, nil
}

func (f *fileRepo) DeleteByID(ctx context.Context, id string) error {
	_, err := f.db.Exec(ctx, `DELETE FROM files WHERE id = $1`, id)
	return err
}
