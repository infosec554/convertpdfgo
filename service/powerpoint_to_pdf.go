package service

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/cache"
	"convertpdfgo/pkg/gotenberg"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/storage"
)

type PowerPointToPDFService interface {
	Create(ctx context.Context, req models.PowerPointToPDFRequest, userID *string) (string, error)
	Process(ctx context.Context, jobID string) error
	GetByID(ctx context.Context, id string) (*models.PowerPointToPDFJob, error)
}

type powerPointToPDFService struct {
	stg       storage.IStorage
	log       logger.ILogger
	gotClient gotenberg.Client
	cache     *cache.ResultCache
}

func NewPowerPointToPDFService(stg storage.IStorage, log logger.ILogger, gotClient gotenberg.Client, c *cache.ResultCache) PowerPointToPDFService {
	return &powerPointToPDFService{stg: stg, log: log, gotClient: gotClient, cache: c}
}

func (s *powerPointToPDFService) Create(ctx context.Context, req models.PowerPointToPDFRequest, userID *string) (string, error) {
	job := &models.PowerPointToPDFJob{
		ID:          uuid.NewString(),
		UserID:      userID,
		InputFileID: req.InputFileID,
		Status:      "pending",
		CreatedAt:   time.Now(),
	}
	if err := s.stg.PowerPointToPDF().Create(ctx, job); err != nil {
		return "", err
	}
	return job.ID, nil
}

func (s *powerPointToPDFService) Process(ctx context.Context, jobID string) error {
	job, err := s.stg.PowerPointToPDF().GetByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("get job: %w", err)
	}
	job.Status = "processing"
	_ = s.stg.PowerPointToPDF().Update(ctx, job)

	file, err := s.stg.File().GetByID(ctx, job.InputFileID)
	if err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("get input file: %w", err)
	}

	if file.SHA256 != nil {
		key := cache.ConvertKey("ppt_to_pdf", *file.SHA256)
		if existingID, cerr := s.cache.Get(ctx, "ppt_to_pdf", key); cerr == nil {
			job.OutputFileID = &existingID
			job.Status = "done"
			return s.stg.PowerPointToPDF().Update(ctx, job)
		} else if !errors.Is(cerr, cache.ErrMiss) {
			s.log.Error("ppt_to_pdf: cache lookup", logger.Error(cerr))
		}
	}

	outputFileID := uuid.NewString()
	outputPath := filepath.Join("storage/powerpoint_to_pdf", outputFileID+".pdf")

	size, err := s.gotClient.PowerPointToPDF(ctx, file.FilePath, outputPath)
	if err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("gotenberg: %w", err)
	}

	if _, err := s.stg.File().Save(ctx, models.File{
		ID:         outputFileID,
		UserID:     job.UserID,
		FileName:   filepath.Base(outputPath),
		FilePath:   outputPath,
		FileType:   "application/pdf",
		FileSize:   size,
		UploadedAt: time.Now(),
	}); err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("save output: %w", err)
	}

	if file.SHA256 != nil {
		_ = s.cache.Set(ctx, cache.ConvertKey("ppt_to_pdf", *file.SHA256), outputFileID, cacheTTL)
	}

	job.OutputFileID = &outputFileID
	job.Status = "done"
	return s.stg.PowerPointToPDF().Update(ctx, job)
}

func (s *powerPointToPDFService) markFailed(ctx context.Context, job *models.PowerPointToPDFJob) {
	job.Status = "failed"
	if err := s.stg.PowerPointToPDF().Update(ctx, job); err != nil {
		s.log.Error("ppt_to_pdf: mark failed", logger.Error(err))
	}
}

func (s *powerPointToPDFService) GetByID(ctx context.Context, id string) (*models.PowerPointToPDFJob, error) {
	return s.stg.PowerPointToPDF().GetByID(ctx, id)
}
