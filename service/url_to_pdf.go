package service

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/gotenberg"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/storage"
)

type URLToPDFService interface {
	Create(ctx context.Context, req models.URLToPDFRequest, userID *string) (string, error)
	Process(ctx context.Context, jobID string) error
	GetByID(ctx context.Context, id string) (*models.URLToPDFJob, error)
}

type urlToPDFService struct {
	stg storage.IStorage
	log logger.ILogger
	got gotenberg.Client
}

func NewURLToPDFService(stg storage.IStorage, log logger.ILogger, got gotenberg.Client) URLToPDFService {
	return &urlToPDFService{stg: stg, log: log, got: got}
}

func (s *urlToPDFService) Create(ctx context.Context, req models.URLToPDFRequest, userID *string) (string, error) {
	u := strings.TrimSpace(req.URL)
	parsed, err := url.Parse(u)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", fmt.Errorf("url must be a full http(s) URL")
	}
	page := req.PageSize
	if page == "" {
		page = "A4"
	}
	job := &models.URLToPDFJob{
		ID:        uuid.NewString(),
		UserID:    userID,
		URL:       u,
		PageSize:  page,
		Landscape: req.Landscape,
		Status:    models.Pending,
		CreatedAt: time.Now(),
	}
	if w := strings.TrimSpace(req.WaitFor); w != "" {
		job.WaitFor = &w
	}
	if err := s.stg.URLToPDF().Create(ctx, job); err != nil {
		return "", err
	}
	return job.ID, nil
}

func (s *urlToPDFService) Process(ctx context.Context, jobID string) error {
	job, err := s.stg.URLToPDF().GetByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("get job: %w", err)
	}

	outID := uuid.NewString()
	outPath := filepath.Join("storage/url_to_pdf", outID+".pdf")

	opts := gotenberg.URLToPDFOptions{
		URL:       job.URL,
		PageSize:  job.PageSize,
		Landscape: job.Landscape,
	}
	if job.WaitFor != nil {
		opts.WaitFor = *job.WaitFor
	}

	size, err := s.got.URLToPDF(ctx, outPath, opts)
	if err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("gotenberg: %w", err)
	}

	if _, err := s.stg.File().Save(ctx, models.File{
		ID:         outID,
		UserID:     job.UserID,
		FileName:   filepath.Base(outPath),
		FilePath:   outPath,
		FileType:   "application/pdf",
		FileSize:   size,
		UploadedAt: time.Now(),
	}); err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("save output: %w", err)
	}

	job.OutputFileID = &outID
	job.Status = models.Done
	return s.stg.URLToPDF().Update(ctx, job)
}

func (s *urlToPDFService) markFailed(ctx context.Context, job *models.URLToPDFJob) {
	job.Status = models.Failed
	if err := s.stg.URLToPDF().Update(ctx, job); err != nil {
		s.log.Error("url_to_pdf: mark failed", logger.Error(err))
	}
}

func (s *urlToPDFService) GetByID(ctx context.Context, id string) (*models.URLToPDFJob, error) {
	return s.stg.URLToPDF().GetByID(ctx, id)
}
