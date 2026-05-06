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

type UnlockService interface {
	Create(ctx context.Context, req models.UnlockPDFRequest, userID *string) (string, error)
	// Process needs the password too because UnlockPDFJob does not store it
	// (we deliberately keep secrets out of the database row).
	Process(ctx context.Context, jobID, password string) error
	GetByID(ctx context.Context, id string) (*models.UnlockPDFJob, error)
}

type unlockService struct {
	stg storage.IStorage
	log logger.ILogger
}

func NewUnlockService(stg storage.IStorage, log logger.ILogger) UnlockService {
	return &unlockService{stg: stg, log: log}
}

func (s *unlockService) Create(ctx context.Context, req models.UnlockPDFRequest, userID *string) (string, error) {
	job := &models.UnlockPDFJob{
		ID:          uuid.NewString(),
		UserID:      userID,
		InputFileID: req.InputFileID,
		Status:      "pending",
		CreatedAt:   time.Now(),
	}
	if err := s.stg.Unlock().Create(ctx, job); err != nil {
		return "", err
	}
	return job.ID, nil
}

func (s *unlockService) Process(ctx context.Context, jobID, password string) error {
	job, err := s.stg.Unlock().GetByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("get job: %w", err)
	}
	job.Status = "processing"
	_ = s.stg.Unlock().Update(ctx, job)

	file, err := s.stg.File().GetByID(ctx, job.InputFileID)
	if err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("get input file: %w", err)
	}

	outputFileID := uuid.NewString()
	outputPath := filepath.Join("storage/unlock_pdf", outputFileID+".pdf")
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("mkdir: %w", err)
	}

	cmd := exec.CommandContext(ctx, "pdfcpu", "decrypt", "-upw", password, file.FilePath, outputPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("pdfcpu decrypt: %w (%s)", err, string(out))
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
	return s.stg.Unlock().Update(ctx, job)
}

func (s *unlockService) markFailed(ctx context.Context, job *models.UnlockPDFJob) {
	job.Status = "failed"
	if err := s.stg.Unlock().Update(ctx, job); err != nil {
		s.log.Error("unlock: mark failed", logger.Error(err))
	}
}

func (s *unlockService) GetByID(ctx context.Context, id string) (*models.UnlockPDFJob, error) {
	return s.stg.Unlock().GetByID(ctx, id)
}
