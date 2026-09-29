package postgres

import (
	"context"
	"fmt"
	"strings"

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
	// Defensively coerce empty *string into nil so callers that hand us a
	// pointer to "" (the qr-code / header-footer pattern of `uid := job.UserID; UserID: &uid`)
	// don't blow up against the UUID column.
	var userID interface{}
	if file.UserID != nil && *file.UserID != "" {
		userID = *file.UserID
	}
	_, err := f.db.Exec(ctx, query,
		file.ID, userID, file.FileName, file.FilePath,
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

func (f *fileRepo) ListByUser(ctx context.Context, userID, q string) ([]models.File, error) {
	// $2 is matched with ILIKE against the file name. An empty q makes the
	// pattern '%%', which every name matches — so one query serves both the
	// plain listing and the search, and the planner still uses the user_id
	// index either way.
	query := `
		SELECT id, user_id, file_name, file_path, file_type, file_size, sha256, uploaded_at
		FROM files
		WHERE user_id = $1 AND file_name ILIKE $2
		ORDER BY uploaded_at DESC
	`
	rows, err := f.db.Query(ctx, query, userID, "%"+q+"%")
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

func (f *fileRepo) FindBySHA256(ctx context.Context, sha string, userID *string) (models.File, error) {
	// Dedup is scoped to the uploader's own slice: guests dedup against
	// guest rows (user_id IS NULL), authenticated users dedup against
	// their own rows. Crossing the boundary would silently expose one
	// user's "private" upload through another user's reused row.
	var (
		file  models.File
		query string
		args  []interface{}
	)
	if userID == nil {
		query = `
			SELECT id, user_id, file_name, file_path, file_type, file_size, sha256, uploaded_at
			FROM files
			WHERE sha256 = $1 AND user_id IS NULL
			ORDER BY uploaded_at DESC
			LIMIT 1
		`
		args = []interface{}{sha}
	} else {
		query = `
			SELECT id, user_id, file_name, file_path, file_type, file_size, sha256, uploaded_at
			FROM files
			WHERE sha256 = $1 AND user_id = $2
			ORDER BY uploaded_at DESC
			LIMIT 1
		`
		args = []interface{}{sha, *userID}
	}
	err := f.db.QueryRow(ctx, query, args...).Scan(
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
		WHERE uploaded_at < NOW() - ( $1::int || ' days')::INTERVAL
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
		  AND uploaded_at < NOW() - ( $1::int || ' minutes')::INTERVAL
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

// GetFilesOlderThanMinutes returns every file (guest + registered) older
// than `minutes`. The 5-minute privacy cleaner uses this; the older
// guest-only variant is kept for callers that still want that filter.
func (f *fileRepo) GetFilesOlderThanMinutes(ctx context.Context, minutes int) ([]models.OldFile, error) {
	const q = `
		SELECT id, file_path
		FROM files
		WHERE uploaded_at < NOW() - ( $1::int || ' minutes')::INTERVAL
	`
	rows, err := f.db.Query(ctx, q, minutes)
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

// ListAll is the admin-side listing. Filter values are optional; pass
// "guest" as UserID to limit to anonymous uploads. Returns rows + total
// matching count so the UI can render pagination.
//
// We build the WHERE incrementally — each clause adds a $N placeholder so
// pgx still uses the prepared-statement cache.
func (f *fileRepo) ListAll(ctx context.Context, fl models.AdminFileFilter) ([]models.File, int, error) {
	limit := fl.Limit
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	offset := fl.Offset
	if offset < 0 {
		offset = 0
	}

	conds := []string{"1=1"}
	args := []any{}
	switch fl.UserID {
	case "":
		// any user
	case "guest":
		conds = append(conds, "user_id IS NULL")
	default:
		args = append(args, fl.UserID)
		conds = append(conds, fmt.Sprintf("user_id = $%d", len(args)))
	}
	if fl.FileType != "" {
		args = append(args, fl.FileType)
		conds = append(conds, fmt.Sprintf("file_type = $%d", len(args)))
	}
	if fl.Q != "" {
		args = append(args, "%"+fl.Q+"%")
		conds = append(conds, fmt.Sprintf("file_name ILIKE $%d", len(args)))
	}
	where := strings.Join(conds, " AND ")

	// Total — same WHERE, no LIMIT/OFFSET.
	var total int
	if err := f.db.QueryRow(ctx,
		"SELECT COUNT(*) FROM files WHERE "+where, args...,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, limit, offset)
	listQ := fmt.Sprintf(`
		SELECT id, user_id, file_name, file_path, file_type, file_size, sha256, uploaded_at
		FROM files
		WHERE %s
		ORDER BY uploaded_at DESC
		LIMIT $%d OFFSET $%d
	`, where, len(args)-1, len(args))

	rows, err := f.db.Query(ctx, listQ, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]models.File, 0, limit)
	for rows.Next() {
		var file models.File
		if err := rows.Scan(&file.ID, &file.UserID, &file.FileName, &file.FilePath,
			&file.FileType, &file.FileSize, &file.SHA256, &file.UploadedAt); err != nil {
			f.log.Error("files: scan ListAll", logger.Error(err))
			continue
		}
		out = append(out, file)
	}
	return out, total, nil
}
