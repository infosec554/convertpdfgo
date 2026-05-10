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

type ExcelToPDFService interface {
	Create(ctx context.Context, req models.ExcelToPDFRequest, userID *string) (string, error)
	Process(ctx context.Context, jobID string) error
	GetByID(ctx context.Context, id string) (*models.ExcelToPDFJob, error)
}

type excelToPDFService struct {
	stg       storage.IStorage
	log       logger.ILogger
	gotClient gotenberg.Client
	cache     *cache.ResultCache
}

func NewExcelToPDFService(stg storage.IStorage, log logger.ILogger, gotClient gotenberg.Client, c *cache.ResultCache) ExcelToPDFService {
	return &excelToPDFService{stg: stg, log: log, gotClient: gotClient, cache: c}
}

func (s *excelToPDFService) Create(ctx context.Context, req models.ExcelToPDFRequest, userID *string) (string, error) {
	job := &models.ExcelToPDFJob{
		ID:          uuid.NewString(),
		UserID:      userID,
		InputFileID: req.InputFileID,
		Status:      "pending",
		CreatedAt:   time.Now(),
	}
	if err := s.stg.ExcelToPDF().Create(ctx, job); err != nil {
		return "", err
	}
	return job.ID, nil
}

func (s *excelToPDFService) Process(ctx context.Context, jobID string) error {
	job, err := s.stg.ExcelToPDF().GetByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("get job: %w", err)
	}
	file, err := s.stg.File().GetByID(ctx, job.InputFileID)
	if err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("get input file: %w", err)
	}

	if file.SHA256 != nil {
		key := cache.ConvertKey("excel_to_pdf", *file.SHA256)
		if existingID, cerr := s.cache.Get(ctx, "excel_to_pdf", key); cerr == nil {
			if _, gerr := s.stg.File().GetByID(ctx, existingID); gerr == nil {
				job.OutputFileID = &existingID
				job.Status = "done"
				return s.stg.ExcelToPDF().Update(ctx, job)
			}
			// stale cache → regenerate below
		} else if !errors.Is(cerr, cache.ErrMiss) {
			s.log.Error("excel_to_pdf: cache lookup", logger.Error(cerr))
		}
	}

	outputFileID := uuid.NewString()
	outputPath := filepath.Join("storage/excel_to_pdf", outputFileID+".pdf")

	size, err := s.gotClient.ExcelToPDF(ctx, file.FilePath, outputPath)
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
		_ = s.cache.Set(ctx, cache.ConvertKey("excel_to_pdf", *file.SHA256), outputFileID, cacheTTL)
	}

	job.OutputFileID = &outputFileID
	job.Status = "done"
	return s.stg.ExcelToPDF().Update(ctx, job)
}

func (s *excelToPDFService) markFailed(ctx context.Context, job *models.ExcelToPDFJob) {
	job.Status = "failed"
	if err := s.stg.ExcelToPDF().Update(ctx, job); err != nil {
		s.log.Error("excel_to_pdf: mark failed", logger.Error(err))
	}
}

func (s *excelToPDFService) GetByID(ctx context.Context, id string) (*models.ExcelToPDFJob, error) {
	return s.stg.ExcelToPDF().GetByID(ctx, id)
}
