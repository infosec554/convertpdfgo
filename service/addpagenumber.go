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

type AddPageNumberService interface {
	Create(ctx context.Context, req models.AddPageNumbersRequest, userID *string) (string, error)
	Process(ctx context.Context, jobID string) error
	GetByID(ctx context.Context, id string) (*models.AddPageNumberJob, error)
}

type addPageNumberService struct {
	stg storage.IStorage
	log logger.ILogger
}

func NewAddPageNumberService(stg storage.IStorage, log logger.ILogger) AddPageNumberService {
	return &addPageNumberService{stg: stg, log: log}
}

func (s *addPageNumberService) Create(ctx context.Context, req models.AddPageNumbersRequest, userID *string) (string, error) {
	job := &models.AddPageNumberJob{
		ID:          uuid.NewString(),
		UserID:      userID,
		InputFileID: req.InputFileID,
		Status:      "pending",
		FirstNumber: req.FirstNumber,
		PageRange:   req.PageRange,
		Position:    req.Position,
		Color:       req.Color,
		FontSize:    req.FontSize,
		CreatedAt:   time.Now(),
	}
	if err := s.stg.AddPageNumber().Create(ctx, job); err != nil {
		return "", err
	}
	return job.ID, nil
}

func (s *addPageNumberService) Process(ctx context.Context, jobID string) error {
	job, err := s.stg.AddPageNumber().GetByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("get job: %w", err)
	}
	job.Status = "processing"
	_ = s.stg.AddPageNumber().Update(ctx, job)

	file, err := s.stg.File().GetByID(ctx, job.InputFileID)
	if err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("get input file: %w", err)
	}

	outputFileID := uuid.NewString()
	outputPath := filepath.Join("storage/add_page_numbers", outputFileID+".pdf")
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("mkdir: %w", err)
	}

	args := []string{
		"stamp", "add",
		"-mode", "text",
		"-pages", job.PageRange,
		"--",
		"Page %p of %P",
		fmt.Sprintf("scale:1.0 abs, pos:%s, rot:0, fillcolor:%s, fontname:Helvetica, points:%d",
			job.Position, job.Color, job.FontSize),
		file.FilePath,
		outputPath,
	}

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
	return s.stg.AddPageNumber().Update(ctx, job)
}

func (s *addPageNumberService) markFailed(ctx context.Context, job *models.AddPageNumberJob) {
	job.Status = "failed"
	if err := s.stg.AddPageNumber().Update(ctx, job); err != nil {
		s.log.Error("addpagenumber: mark failed", logger.Error(err))
	}
}

func (s *addPageNumberService) GetByID(ctx context.Context, id string) (*models.AddPageNumberJob, error) {
	return s.stg.AddPageNumber().GetByID(ctx, id)
}
