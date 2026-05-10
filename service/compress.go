package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/cache"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/storage"
)

const cacheTTL = 24 * time.Hour

type CompressService interface {
	Create(ctx context.Context, req models.CompressRequest, userID *string) (string, error)
	Process(ctx context.Context, jobID string) error
	GetByID(ctx context.Context, id string) (*models.CompressJob, error)
}

type compressService struct {
	stg   storage.IStorage
	log   logger.ILogger
	cache *cache.ResultCache
}

func NewCompressService(stg storage.IStorage, log logger.ILogger, c *cache.ResultCache) CompressService {
	return &compressService{stg: stg, log: log, cache: c}
}

func (s *compressService) Create(ctx context.Context, req models.CompressRequest, userID *string) (string, error) {
	job := &models.CompressJob{
		ID:               uuid.NewString(),
		UserID:           userID,
		InputFileID:      req.InputFileID,
		CompressionLevel: req.Compression,
		Status:           models.Pending,
		CreatedAt:        time.Now(),
	}
	if err := s.stg.Compress().Create(ctx, job); err != nil {
		s.log.Error("compress: create job", logger.Error(err))
		return "", err
	}
	return job.ID, nil
}

func (s *compressService) Process(ctx context.Context, jobID string) error {
	job, err := s.stg.Compress().GetByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("get job: %w", err)
	}

	job.Status = models.Processing
	_ = s.stg.Compress().Update(ctx, job)

	file, err := s.stg.File().GetByID(ctx, job.InputFileID)
	if err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("get input file: %w", err)
	}

	// Result cache lookup: identical (sha, level) ⇒ reuse the previous output.
	// Verify the cached file still exists — the privacy cleaner deletes
	// outputs after 5 min, so a hit can point at a row that's already gone.
	if file.SHA256 != nil {
		key := cache.CompressKey(*file.SHA256, string(job.CompressionLevel))
		if existingID, cerr := s.cache.Get(ctx, "compress", key); cerr == nil {
			if _, gerr := s.stg.File().GetByID(ctx, existingID); gerr == nil {
				job.OutputFileID = &existingID
				job.Status = models.Done
				return s.stg.Compress().Update(ctx, job)
			}
			// stale cache → fall through and regenerate
		} else if !errors.Is(cerr, cache.ErrMiss) {
			s.log.Error("compress: cache lookup", logger.Error(cerr))
		}
	}

	outputID := uuid.NewString()
	outputDir := "storage/compress"
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("mkdir: %w", err)
	}
	outputPath := filepath.Join(outputDir, outputID+".pdf")

	conf := model.NewDefaultConfiguration()
	conf.Cmd = model.OPTIMIZE
	if err := api.OptimizeFile(file.FilePath, outputPath, conf); err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("pdfcpu optimize: %w", err)
	}

	fi, err := os.Stat(outputPath)
	if err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("stat output: %w", err)
	}

	out := models.File{
		ID:         outputID,
		UserID:     job.UserID,
		FileName:   filepath.Base(outputPath),
		FilePath:   outputPath,
		FileType:   "application/pdf",
		FileSize:   fi.Size(),
		UploadedAt: time.Now(),
	}
	if _, err := s.stg.File().Save(ctx, out); err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("save output file: %w", err)
	}

	if file.SHA256 != nil {
		key := cache.CompressKey(*file.SHA256, string(job.CompressionLevel))
		_ = s.cache.Set(ctx, key, outputID, cacheTTL)
	}

	job.OutputFileID = &outputID
	job.Status = models.Done
	if err := s.stg.Compress().Update(ctx, job); err != nil {
		return fmt.Errorf("update job: %w", err)
	}
	return nil
}

func (s *compressService) markFailed(ctx context.Context, job *models.CompressJob) {
	job.Status = models.Failed
	if err := s.stg.Compress().Update(ctx, job); err != nil {
		s.log.Error("compress: mark failed", logger.Error(err))
	}
}

func (s *compressService) GetByID(ctx context.Context, id string) (*models.CompressJob, error) {
	return s.stg.Compress().GetByID(ctx, id)
}
