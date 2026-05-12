package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/libreoffice"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/storage"
)

type PDFToHTMLService interface {
	Create(ctx context.Context, req models.PDFToHTMLRequest, userID *string) (string, error)
	Process(ctx context.Context, jobID string) error
	GetByID(ctx context.Context, id string) (*models.PDFToHTMLJob, error)
}

type pdfToHTMLService struct {
	stg storage.IStorage
	log logger.ILogger
}

func NewPDFToHTMLService(stg storage.IStorage, log logger.ILogger) PDFToHTMLService {
	return &pdfToHTMLService{stg: stg, log: log}
}

func (s *pdfToHTMLService) Create(ctx context.Context, req models.PDFToHTMLRequest, userID *string) (string, error) {
	job := &models.PDFToHTMLJob{
		ID:          uuid.NewString(),
		UserID:      userID,
		InputFileID: req.InputFileID,
		Status:      "pending",
		CreatedAt:   time.Now(),
	}
	if err := s.stg.PDFToHTML().Create(ctx, job); err != nil {
		return "", err
	}
	return job.ID, nil
}

func (s *pdfToHTMLService) Process(ctx context.Context, jobID string) error {
	job, err := s.stg.PDFToHTML().GetByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("get job: %w", err)
	}
	file, err := s.stg.File().GetByID(ctx, job.InputFileID)
	if err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("get input file: %w", err)
	}

	outDir := "storage/pdf_to_html"
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("mkdir: %w", err)
	}
	tmpPath, err := libreoffice.Convert(ctx, file.FilePath, outDir, "html")
	if err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("libreoffice: %w", err)
	}
	outputFileID := uuid.NewString()
	outputPath := filepath.Join(outDir, outputFileID+".html")
	if err := os.Rename(tmpPath, outputPath); err != nil {
		_ = os.Remove(tmpPath)
		s.markFailed(ctx, job)
		return fmt.Errorf("rename: %w", err)
	}
	fi, err := os.Stat(outputPath)
	if err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("stat: %w", err)
	}
	if _, err := s.stg.File().Save(ctx, models.File{
		ID:         outputFileID,
		UserID:     job.UserID,
		FileName:   filepath.Base(outputPath),
		FilePath:   outputPath,
		FileType:   "text/html",
		FileSize:   fi.Size(),
		UploadedAt: time.Now(),
	}); err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("save output: %w", err)
	}
	job.OutputFileID = &outputFileID
	job.Status = "done"
	return s.stg.PDFToHTML().Update(ctx, job)
}

func (s *pdfToHTMLService) markFailed(ctx context.Context, job *models.PDFToHTMLJob) {
	job.Status = "failed"
	if err := s.stg.PDFToHTML().Update(ctx, job); err != nil {
		s.log.Error("pdf_to_html: mark failed", logger.Error(err))
	}
}

func (s *pdfToHTMLService) GetByID(ctx context.Context, id string) (*models.PDFToHTMLJob, error) {
	return s.stg.PDFToHTML().GetByID(ctx, id)
}
