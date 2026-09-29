package service

import (
	"context"
	"os"
	"time"

	"github.com/google/uuid"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/pkg/metrics"
	"convertpdfgo/storage"
)

type FileService interface {
	Upload(ctx context.Context, req models.File) (string, error)
	Get(ctx context.Context, id string) (models.File, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, userID, q string) ([]models.File, error)
	CleanupOldFiles(ctx context.Context, olderThanDays int) (int, error)
	CleanupGuestFiles(ctx context.Context, minutes int) (int, error)

	// CleanupAllFiles is the privacy-first cleaner: deletes both guest and
	// registered files once they're older than `minutes`. The default cron
	// runs it at 5-minute retention.
	CleanupAllFiles(ctx context.Context, minutes int) (int, error)

	FindBySHA256(ctx context.Context, sha string, userID *string) (models.File, error)

	// ListAll is the admin-side cross-user listing.
	ListAll(ctx context.Context, filter models.AdminFileFilter) ([]models.File, int, error)
}

type fileService struct {
	stg storage.IFileStorage
	log logger.ILogger
}

func NewFileService(stg storage.IStorage, log logger.ILogger) FileService {
	return &fileService{
		stg: stg.File(), // storageManager.File()
		log: log,
	}
}

// Upload - faylni DBga yozish (upload jarayoni)
func (s *fileService) Upload(ctx context.Context, req models.File) (string, error) {
	req.ID = uuid.NewString()
	req.UploadedAt = time.Now()

	id, err := s.stg.Save(ctx, req)
	if err != nil {
		s.log.Error("failed to save file", logger.Error(err))
		return "", err
	}
	return id, nil
}

// Get - faylni ID orqali olish
func (s *fileService) Get(ctx context.Context, id string) (models.File, error) {
	return s.stg.GetByID(ctx, id)
}

// Delete - faylni o‘chirish
func (s *fileService) Delete(ctx context.Context, id string) error {
	return s.stg.Delete(ctx, id)
}

// List - user fayllari ro‘yxati. q bo‘sh bo‘lsa — hammasi.
func (s *fileService) List(ctx context.Context, userID, q string) ([]models.File, error) {
	return s.stg.ListByUser(ctx, userID, q)
}

// FindBySHA256 returns the most recent existing file with the given hash
// owned by the caller. nil userID means "the guest slice".
func (s *fileService) FindBySHA256(ctx context.Context, sha string, userID *string) (models.File, error) {
	return s.stg.FindBySHA256(ctx, sha, userID)
}

// ListAll — admin-side cross-user file listing with pagination.
func (s *fileService) ListAll(ctx context.Context, filter models.AdminFileFilter) ([]models.File, int, error) {
	return s.stg.ListAll(ctx, filter)
}

// CleanupGuestFiles removes files belonging to anonymous (NULL user_id)
// uploads older than the given minute threshold. Disk + DB row are both
// removed. Used by the fast cleaner so guest data doesn't accumulate.
func (s *fileService) CleanupGuestFiles(ctx context.Context, minutes int) (int, error) {
	files, err := s.stg.GetGuestFilesOlderThanMinutes(ctx, minutes)
	if err != nil {
		s.log.Error("cleanup guest: list", logger.Error(err))
		return 0, err
	}
	count := 0
	for _, f := range files {
		if err := os.Remove(f.FilePath); err != nil && !os.IsNotExist(err) {
			metrics.SilentError("cleaner.disk_remove")
			s.log.Warning("cleanup guest: remove disk", logger.Error(err), logger.String("path", f.FilePath))
		}
		if err := s.stg.DeleteByID(ctx, f.ID); err != nil {
			metrics.SilentError("cleaner.db_remove")
			s.log.Warning("cleanup guest: remove db", logger.Error(err), logger.String("id", f.ID))
			continue
		}
		count++
	}
	return count, nil
}

// CleanupAllFiles deletes every file (guest + registered) older than
// `minutes`. Migration 0008 made the *_jobs FKs CASCADE so the DB row
// removal here also wipes any referencing job rows in one shot.
func (s *fileService) CleanupAllFiles(ctx context.Context, minutes int) (int, error) {
	files, err := s.stg.GetFilesOlderThanMinutes(ctx, minutes)
	if err != nil {
		s.log.Error("cleanup all: list", logger.Error(err))
		return 0, err
	}
	count := 0
	for _, f := range files {
		if err := os.Remove(f.FilePath); err != nil && !os.IsNotExist(err) {
			metrics.SilentError("cleaner.disk_remove")
			s.log.Warning("cleanup all: remove disk", logger.Error(err), logger.String("path", f.FilePath))
		}
		if err := s.stg.DeleteByID(ctx, f.ID); err != nil {
			metrics.SilentError("cleaner.db_remove")
			s.log.Warning("cleanup all: remove db", logger.Error(err), logger.String("id", f.ID))
			continue
		}
		count++
	}
	return count, nil
}

func (s *fileService) CleanupOldFiles(ctx context.Context, olderThanDays int) (int, error) {
	oldFiles, err := s.stg.GetOldFiles(ctx, olderThanDays)
	if err != nil {
		s.log.Error("failed to get old files", logger.Error(err))
		return 0, err
	}

	count := 0
	for _, file := range oldFiles {
		if err := os.Remove(file.FilePath); err != nil {
			s.log.Error("failed to delete file from disk", logger.Error(err), logger.String("path", file.FilePath))
			continue
		}
		if err := s.stg.DeleteByID(ctx, file.ID); err != nil {
			s.log.Error("failed to delete file from db", logger.Error(err), logger.String("id", file.ID))
			continue
		}
		count++
	}

	return count, nil
}
