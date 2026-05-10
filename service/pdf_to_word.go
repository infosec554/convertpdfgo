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

type PDFToWordService interface {
	Create(ctx context.Context, req models.PDFToWordRequest, userID *string) (string, error)
	Process(ctx context.Context, jobID string) error
	GetByID(ctx context.Context, id string) (*models.PDFToWordJob, error)
}

type pdfToWordService struct {
	stg       storage.IStorage
	log       logger.ILogger
	gotClient gotenberg.Client
	cache     *cache.ResultCache
}

func NewPDFToWordService(stg storage.IStorage, log logger.ILogger, gotClient gotenberg.Client, c *cache.ResultCache) PDFToWordService {
	return &pdfToWordService{stg: stg, log: log, gotClient: gotClient, cache: c}
}

func (s *pdfToWordService) Create(ctx context.Context, req models.PDFToWordRequest, userID *string) (string, error) {
	job := &models.PDFToWordJob{
		ID:          uuid.NewString(),
		UserID:      userID,
		InputFileID: req.InputFileID,
		Status:      "pending",
		CreatedAt:   time.Now(),
	}
	if err := s.stg.PDFToWord().Create(ctx, job); err != nil {
		return "", err
	}
	return job.ID, nil
}

func (s *pdfToWordService) Process(ctx context.Context, jobID string) error {
	job, err := s.stg.PDFToWord().GetByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("get job: %w", err)
	}
	file, err := s.stg.File().GetByID(ctx, job.InputFileID)
	if err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("get input file: %w", err)
	}

	if file.SHA256 != nil {
		key := cache.ConvertKey("pdf_to_word", *file.SHA256)
		if existingID, cerr := s.cache.Get(ctx, "pdf_to_word", key); cerr == nil {
			job.OutputFileID = &existingID
			job.Status = "done"
			return s.stg.PDFToWord().Update(ctx, job)
		} else if !errors.Is(cerr, cache.ErrMiss) {
			s.log.Error("pdf_to_word: cache lookup", logger.Error(cerr))
		}
	}

	outputFileID := uuid.NewString()
	outputPath := filepath.Join("storage/pdf_to_word", outputFileID+".docx")

	size, err := s.gotClient.PDFToWord(ctx, file.FilePath, outputPath)
	if err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("gotenberg: %w", err)
	}

	if _, err := s.stg.File().Save(ctx, models.File{
		ID:         outputFileID,
		UserID:     job.UserID,
		FileName:   filepath.Base(outputPath),
		FilePath:   outputPath,
		FileType:   "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		FileSize:   size,
		UploadedAt: time.Now(),
	}); err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("save output: %w", err)
	}

	if file.SHA256 != nil {
		_ = s.cache.Set(ctx, cache.ConvertKey("pdf_to_word", *file.SHA256), outputFileID, cacheTTL)
	}

	job.OutputFileID = &outputFileID
	job.Status = "done"
	return s.stg.PDFToWord().Update(ctx, job)
}

func (s *pdfToWordService) markFailed(ctx context.Context, job *models.PDFToWordJob) {
	job.Status = "failed"
	if err := s.stg.PDFToWord().Update(ctx, job); err != nil {
		s.log.Error("pdf_to_word: mark failed", logger.Error(err))
	}
}

func (s *pdfToWordService) GetByID(ctx context.Context, id string) (*models.PDFToWordJob, error) {
	return s.stg.PDFToWord().GetByID(ctx, id)
}
