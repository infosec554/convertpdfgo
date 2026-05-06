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

type ProtectPDFService interface {
	Create(ctx context.Context, req models.ProtectPDFRequest, userID *string) (string, error)
	Process(ctx context.Context, jobID string) error
	GetByID(ctx context.Context, id string) (*models.ProtectPDFJob, error)
}

type protectPDFService struct {
	stg storage.IStorage
	log logger.ILogger
}

func NewProtectPDFService(stg storage.IStorage, log logger.ILogger) ProtectPDFService {
	return &protectPDFService{stg: stg, log: log}
}

func (s *protectPDFService) Create(ctx context.Context, req models.ProtectPDFRequest, userID *string) (string, error) {
	job := &models.ProtectPDFJob{
		ID:          uuid.NewString(),
		UserID:      userID,
		InputFileID: req.InputFileID,
		Password:    req.Password,
		Status:      "pending",
		CreatedAt:   time.Now(),
	}
	if err := s.stg.Protect().Create(ctx, job); err != nil {
		return "", err
	}
	return job.ID, nil
}

func (s *protectPDFService) Process(ctx context.Context, jobID string) error {
	job, err := s.stg.Protect().GetByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("get job: %w", err)
	}
	job.Status = "processing"
	_ = s.stg.Protect().Update(ctx, job)

	file, err := s.stg.File().GetByID(ctx, job.InputFileID)
	if err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("get input file: %w", err)
	}

	outputFileID := uuid.NewString()
	outputPath := filepath.Join("storage/protect_pdf", outputFileID+".pdf")
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("mkdir: %w", err)
	}

	cmd := exec.CommandContext(ctx, "pdfcpu", "encrypt", "-upw", job.Password, "-opw", job.Password, file.FilePath, outputPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("pdfcpu encrypt: %w (%s)", err, string(out))
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
	return s.stg.Protect().Update(ctx, job)
}

func (s *protectPDFService) markFailed(ctx context.Context, job *models.ProtectPDFJob) {
	job.Status = "failed"
	if err := s.stg.Protect().Update(ctx, job); err != nil {
		s.log.Error("protect: mark failed", logger.Error(err))
	}
}

func (s *protectPDFService) GetByID(ctx context.Context, id string) (*models.ProtectPDFJob, error) {
	return s.stg.Protect().GetByID(ctx, id)
}
