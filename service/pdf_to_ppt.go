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

type PDFToPPTService interface {
	Create(ctx context.Context, req models.PDFToPPTRequest, userID *string) (string, error)
	Process(ctx context.Context, jobID string) error
	GetByID(ctx context.Context, id string) (*models.PDFToPPTJob, error)
}

type pdfToPPTService struct {
	stg storage.IStorage
	log logger.ILogger
}

func NewPDFToPPTService(stg storage.IStorage, log logger.ILogger) PDFToPPTService {
	return &pdfToPPTService{stg: stg, log: log}
}

func (s *pdfToPPTService) Create(ctx context.Context, req models.PDFToPPTRequest, userID *string) (string, error) {
	job := &models.PDFToPPTJob{
		ID:          uuid.NewString(),
		UserID:      userID,
		InputFileID: req.InputFileID,
		Status:      "pending",
		CreatedAt:   time.Now(),
	}
	if err := s.stg.PDFToPPT().Create(ctx, job); err != nil {
		return "", err
	}
	return job.ID, nil
}

func (s *pdfToPPTService) Process(ctx context.Context, jobID string) error {
	job, err := s.stg.PDFToPPT().GetByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("get job: %w", err)
	}
	file, err := s.stg.File().GetByID(ctx, job.InputFileID)
	if err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("get input file: %w", err)
	}

	outDir := "storage/pdf_to_ppt"
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("mkdir: %w", err)
	}
	// "pptx" routes through pkg/libreoffice.pdfInputFilterFor →
	// impress_pdf_import, so LibreOffice opens the PDF in Impress and
	// each page becomes one slide.
	tmpPath, err := libreoffice.Convert(ctx, file.FilePath, outDir, "pptx")
	if err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("libreoffice: %w", err)
	}
	outputFileID := uuid.NewString()
	outputPath := filepath.Join(outDir, outputFileID+".pptx")
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
		FileType:   "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		FileSize:   fi.Size(),
		UploadedAt: time.Now(),
	}); err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("save output: %w", err)
	}
	job.OutputFileID = &outputFileID
	job.Status = "done"
	return s.stg.PDFToPPT().Update(ctx, job)
}

func (s *pdfToPPTService) markFailed(ctx context.Context, job *models.PDFToPPTJob) {
	job.Status = "failed"
	if err := s.stg.PDFToPPT().Update(ctx, job); err != nil {
		s.log.Error("pdf_to_ppt: mark failed", logger.Error(err))
	}
}

func (s *pdfToPPTService) GetByID(ctx context.Context, id string) (*models.PDFToPPTJob, error) {
	return s.stg.PDFToPPT().GetByID(ctx, id)
}
