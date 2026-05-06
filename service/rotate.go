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

type RotateService interface {
	Create(ctx context.Context, req models.RotatePDFRequest, userID *string) (string, error)
	Process(ctx context.Context, jobID string) error
	GetByID(ctx context.Context, id string) (*models.RotateJob, error)
}

type rotateService struct {
	stg storage.IStorage
	log logger.ILogger
}

func NewRotateService(stg storage.IStorage, log logger.ILogger) RotateService {
	return &rotateService{stg: stg, log: log}
}

func (s *rotateService) Create(ctx context.Context, req models.RotatePDFRequest, userID *string) (string, error) {
	job := &models.RotateJob{
		ID:          uuid.NewString(),
		UserID:      userID,
		InputFileID: req.InputFileID,
		Status:      "pending",
		Angle:       req.Angle,
		Pages:       req.Pages,
		CreatedAt:   time.Now(),
	}
	if err := s.stg.Rotate().Create(ctx, job); err != nil {
		return "", err
	}
	return job.ID, nil
}

func (s *rotateService) Process(ctx context.Context, jobID string) error {
	job, err := s.stg.Rotate().GetByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("get job: %w", err)
	}
	job.Status = "processing"
	_ = s.stg.Rotate().Update(ctx, job)

	file, err := s.stg.File().GetByID(ctx, job.InputFileID)
	if err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("get input file: %w", err)
	}

	outputFileID := uuid.NewString()
	outputPath := filepath.Join("storage/rotate_files", outputFileID+".pdf")
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("mkdir: %w", err)
	}

	args := []string{"rotate"}
	if job.Pages != "all" && job.Pages != "" {
		args = append(args, "-pages", job.Pages)
	}
	args = append(args, file.FilePath, fmt.Sprintf("%d", job.Angle), outputPath)

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
	return s.stg.Rotate().Update(ctx, job)
}

func (s *rotateService) markFailed(ctx context.Context, job *models.RotateJob) {
	job.Status = "failed"
	if err := s.stg.Rotate().Update(ctx, job); err != nil {
		s.log.Error("rotate: mark failed", logger.Error(err))
	}
}

func (s *rotateService) GetByID(ctx context.Context, id string) (*models.RotateJob, error) {
	return s.stg.Rotate().GetByID(ctx, id)
}
