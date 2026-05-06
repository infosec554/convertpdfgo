package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/jung-kurt/gofpdf"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/storage"
)

type JPGToPDFService interface {
	CreateJob(ctx context.Context, userID *string, inputFileIDs []string) (string, error)
	Process(ctx context.Context, jobID string) error
	GetJobByID(ctx context.Context, id string) (*models.JPGToPDFJob, error)
}

type jpgToPDFService struct {
	stg storage.IStorage
	log logger.ILogger
}

func NewJPGToPDFService(stg storage.IStorage, log logger.ILogger) JPGToPDFService {
	return &jpgToPDFService{stg: stg, log: log}
}

func (s *jpgToPDFService) CreateJob(ctx context.Context, userID *string, inputFileIDs []string) (string, error) {
	if len(inputFileIDs) == 0 {
		return "", fmt.Errorf("no input files provided")
	}
	job := &models.JPGToPDFJob{
		ID:           uuid.NewString(),
		UserID:       userID,
		InputFileIDs: inputFileIDs,
		Status:       "pending",
		CreatedAt:    time.Now(),
	}
	if err := s.stg.JPGToPDF().Create(ctx, job); err != nil {
		return "", err
	}
	return job.ID, nil
}

func (s *jpgToPDFService) Process(ctx context.Context, jobID string) error {
	job, err := s.stg.JPGToPDF().GetByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("get job: %w", err)
	}

	var inputPaths []string
	for _, fileID := range job.InputFileIDs {
		file, err := s.stg.File().GetByID(ctx, fileID)
		if err != nil {
			s.markFailed(ctx, job)
			return fmt.Errorf("input file %s: %w", fileID, err)
		}
		inputPaths = append(inputPaths, file.FilePath)
	}

	outputID := uuid.NewString()
	outputDir := filepath.Join("storage", "jpg_to_pdf")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("mkdir: %w", err)
	}
	outputPath := filepath.Join(outputDir, outputID+".pdf")

	pdf := gofpdf.New("P", "mm", "A4", "")
	for _, imgPath := range inputPaths {
		pdf.AddPage()
		pdf.ImageOptions(imgPath, 10, 10, 190, 0, false, gofpdf.ImageOptions{ImageType: "JPG", ReadDpi: true}, 0, "")
	}
	if err := pdf.OutputFileAndClose(outputPath); err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("gofpdf: %w", err)
	}

	fi, err := os.Stat(outputPath)
	if err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("stat output: %w", err)
	}

	if _, err := s.stg.File().Save(ctx, models.File{
		ID:         outputID,
		UserID:     job.UserID,
		FileName:   filepath.Base(outputPath),
		FilePath:   outputPath,
		FileType:   "application/pdf",
		FileSize:   fi.Size(),
		UploadedAt: time.Now(),
	}); err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("save output: %w", err)
	}

	if err := s.stg.JPGToPDF().UpdateStatusAndOutput(ctx, job.ID, "done", outputID); err != nil {
		return fmt.Errorf("update job: %w", err)
	}
	return nil
}

func (s *jpgToPDFService) markFailed(ctx context.Context, job *models.JPGToPDFJob) {
	if err := s.stg.JPGToPDF().UpdateStatusAndOutput(ctx, job.ID, "failed", ""); err != nil {
		s.log.Error("jpgtopdf: mark failed", logger.Error(err))
	}
}

func (s *jpgToPDFService) GetJobByID(ctx context.Context, id string) (*models.JPGToPDFJob, error) {
	return s.stg.JPGToPDF().GetByID(ctx, id)
}
