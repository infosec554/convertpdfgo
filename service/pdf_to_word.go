package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/cache"
	"convertpdfgo/pkg/libreoffice"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/storage"
)

type PDFToWordService interface {
	Create(ctx context.Context, req models.PDFToWordRequest, userID *string) (string, error)
	Process(ctx context.Context, jobID string) error
	GetByID(ctx context.Context, id string) (*models.PDFToWordJob, error)
}

type pdfToWordService struct {
	stg   storage.IStorage
	log   logger.ILogger
	cache *cache.ResultCache
}

// NewPDFToWordService used to take a gotenberg.Client; Gotenberg's
// libreoffice route is one-way (office→PDF only), so PDF→DOCX now goes
// through pkg/libreoffice (CLI in-image). The arg is kept for backwards
// compatibility with the wiring in service.New(); it's ignored.
func NewPDFToWordService(stg storage.IStorage, log logger.ILogger, _ any, c *cache.ResultCache) PDFToWordService {
	return &pdfToWordService{stg: stg, log: log, cache: c}
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
			if _, gerr := s.stg.File().GetByID(ctx, existingID); gerr == nil {
				job.OutputFileID = &existingID
				job.Status = "done"
				return s.stg.PDFToWord().Update(ctx, job)
			}
			// stale cache → regenerate below
		} else if !errors.Is(cerr, cache.ErrMiss) {
			s.log.Error("pdf_to_word: cache lookup", logger.Error(cerr))
		}
	}

	outDir := "storage/pdf_to_word"
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("mkdir: %w", err)
	}
	// libreoffice picks its own name (input basename + .docx). Convert
	// writes there, then we rename to a UUID so the file row matches.
	tmpPath, err := libreoffice.Convert(ctx, file.FilePath, outDir, "docx")
	if err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("libreoffice: %w", err)
	}
	outputFileID := uuid.NewString()
	outputPath := filepath.Join(outDir, outputFileID+".docx")
	if err := os.Rename(tmpPath, outputPath); err != nil {
		_ = os.Remove(tmpPath)
		s.markFailed(ctx, job)
		return fmt.Errorf("rename: %w", err)
	}
	fi, err := os.Stat(outputPath)
	if err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("stat output: %w", err)
	}
	size := fi.Size()

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
