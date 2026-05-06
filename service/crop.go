package service

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/storage"
)

type CropPDFService interface {
	Create(ctx context.Context, req models.CropPDFRequest, userID *string) (string, error)
	Process(ctx context.Context, jobID string) error
	GetByID(ctx context.Context, id string) (*models.CropPDFJob, error)
}

type cropPDFService struct {
	stg storage.IStorage
	log logger.ILogger
}

func NewCropPDFService(stg storage.IStorage, log logger.ILogger) CropPDFService {
	return &cropPDFService{stg: stg, log: log}
}

func (s *cropPDFService) Create(ctx context.Context, req models.CropPDFRequest, userID *string) (string, error) {
	job := &models.CropPDFJob{
		ID:          uuid.NewString(),
		UserID:      userID,
		InputFileID: req.InputFileID,
		Top:         req.Top,
		Bottom:      req.Bottom,
		Left:        req.Left,
		Right:       req.Right,
		Box:         req.Box,
		Pages:       req.Pages,
		Status:      "pending",
		CreatedAt:   time.Now(),
	}
	if err := s.stg.Crop().Create(ctx, job); err != nil {
		return "", err
	}
	return job.ID, nil
}

func (s *cropPDFService) Process(ctx context.Context, jobID string) error {
	job, err := s.stg.Crop().GetByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("get job: %w", err)
	}
	job.Status = "processing"
	_ = s.stg.Crop().Update(ctx, job)

	file, err := s.stg.File().GetByID(ctx, job.InputFileID)
	if err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("get input file: %w", err)
	}

	outputFileID := uuid.NewString()
	outputPath := filepath.Join("storage/crop_pdfs", outputFileID+".pdf")
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("mkdir: %w", err)
	}

	cropDesc := fmt.Sprintf("%d %d %d %d", job.Top, job.Right, job.Bottom, job.Left)
	args := []string{"crop"}
	if job.Pages != "" {
		args = append(args, "-pages", job.Pages)
	}
	args = append(args, "--", cropDesc, file.FilePath, outputPath)

	cmd := exec.CommandContext(ctx, "pdfcpu", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("pdfcpu: %w (%s)", err, string(out))
	}

	fi, err := os.Stat(outputPath)
	if err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("stat output: %w", err)
	}

	if _, err := s.stg.File().Save(ctx, models.File{
		ID:         outputFileID,
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

	job.OutputFileID = &outputFileID
	job.Status = "done"
	return s.stg.Crop().Update(ctx, job)
}

func (s *cropPDFService) markFailed(ctx context.Context, job *models.CropPDFJob) {
	job.Status = "failed"
	if err := s.stg.Crop().Update(ctx, job); err != nil {
		s.log.Error("crop: mark failed", logger.Error(err))
	}
}

func (s *cropPDFService) GetByID(ctx context.Context, id string) (*models.CropPDFJob, error) {
	return s.stg.Crop().GetByID(ctx, id)
}
