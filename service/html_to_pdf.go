package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/cache"
	"convertpdfgo/pkg/gotenberg"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/storage"
)

type HTMLToPDFService interface {
	Create(ctx context.Context, req models.CreateHTMLToPDFRequest, userID *string) (string, error)
	Process(ctx context.Context, jobID string) error
	GetByID(ctx context.Context, id string) (*models.HTMLToPDFJob, error)
}

type htmlToPDFService struct {
	stg       storage.IStorage
	log       logger.ILogger
	gotClient gotenberg.Client
	cache     *cache.ResultCache
}

func NewHTMLToPDFService(stg storage.IStorage, log logger.ILogger, gotClient gotenberg.Client, c *cache.ResultCache) HTMLToPDFService {
	return &htmlToPDFService{stg: stg, log: log, gotClient: gotClient, cache: c}
}

func (s *htmlToPDFService) Create(ctx context.Context, req models.CreateHTMLToPDFRequest, userID *string) (string, error) {
	job := &models.HTMLToPDFJob{
		ID:          uuid.NewString(),
		UserID:      userID,
		HTMLContent: req.HTMLContent,
		Status:      "pending",
		CreatedAt:   time.Now(),
	}
	if err := s.stg.HTMLToPDF().Create(ctx, job); err != nil {
		return "", err
	}
	return job.ID, nil
}

func (s *htmlToPDFService) Process(ctx context.Context, jobID string) error {
	job, err := s.stg.HTMLToPDF().GetByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("get job: %w", err)
	}
	// Hash the HTML content directly (no input file row).
	h := sha256.Sum256([]byte(job.HTMLContent))
	contentHash := hex.EncodeToString(h[:])

	cacheKey := cache.ConvertKey("html_to_pdf", contentHash)
	if existingID, cerr := s.cache.Get(ctx, "html_to_pdf", cacheKey); cerr == nil {
		if _, gerr := s.stg.File().GetByID(ctx, existingID); gerr == nil {
			job.OutputFileID = &existingID
			job.Status = "done"
			return s.stg.HTMLToPDF().Update(ctx, job)
		}
		// stale cache → regenerate below
	} else if !errors.Is(cerr, cache.ErrMiss) {
		s.log.Error("html_to_pdf: cache lookup", logger.Error(cerr))
	}

	tempID := uuid.NewString()
	htmlFilePath := filepath.Join("tmp", tempID+".html")
	if err := os.MkdirAll("tmp", 0o755); err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("mkdir tmp: %w", err)
	}
	if err := os.WriteFile(htmlFilePath, []byte(job.HTMLContent), 0o644); err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("write html: %w", err)
	}
	defer os.Remove(htmlFilePath)

	outputFileID := uuid.NewString()
	outputPath := filepath.Join("storage/html_to_pdf", outputFileID+".pdf")

	size, err := s.gotClient.HTMLToPDF(ctx, htmlFilePath, outputPath)
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

	_ = s.cache.Set(ctx, cacheKey, outputFileID, cacheTTL)

	job.OutputFileID = &outputFileID
	job.Status = "done"
	return s.stg.HTMLToPDF().Update(ctx, job)
}

func (s *htmlToPDFService) markFailed(ctx context.Context, job *models.HTMLToPDFJob) {
	job.Status = "failed"
	if err := s.stg.HTMLToPDF().Update(ctx, job); err != nil {
		s.log.Error("html_to_pdf: mark failed", logger.Error(err))
	}
}

func (s *htmlToPDFService) GetByID(ctx context.Context, id string) (*models.HTMLToPDFJob, error) {
	return s.stg.HTMLToPDF().GetByID(ctx, id)
}
