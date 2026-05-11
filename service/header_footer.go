package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/addheaderfooter"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/storage"
)

type AddHeaderFooterService interface {
	Create(ctx context.Context, req models.CreateAddHeaderFooterRequest, userID *string) (string, error)
	Process(ctx context.Context, jobID string) error
	GetByID(ctx context.Context, id string) (*models.AddHeaderFooterJob, error)
}

type addHeaderFooterService struct {
	stg storage.IStorage
	log logger.ILogger
}

func NewAddHeaderFooterService(stg storage.IStorage, log logger.ILogger) AddHeaderFooterService {
	return &addHeaderFooterService{stg: stg, log: log}
}

func (s *addHeaderFooterService) Create(ctx context.Context, req models.CreateAddHeaderFooterRequest, userID *string) (string, error) {
	job := &models.AddHeaderFooterJob{
		ID:          uuid.NewString(),
		UserID:      userID,
		InputFileID: req.InputFileID,
		HeaderText:  req.HeaderText,
		FooterText:  req.FooterText,
		FontSize:    req.FontSize,
		FontColor:   req.FontColor,
		Position:    req.Position,
		Status:      "pending",
		CreatedAt:   time.Now(),
	}
	if err := s.stg.AddHeaderFooter().Create(ctx, job); err != nil {
		return "", err
	}
	return job.ID, nil
}

func (s *addHeaderFooterService) Process(ctx context.Context, jobID string) error {
	job, err := s.stg.AddHeaderFooter().GetByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("get job: %w", err)
	}
	file, err := s.stg.File().GetByID(ctx, job.InputFileID)
	if err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("get input file: %w", err)
	}

	outputID := uuid.NewString()
	outputPath := filepath.Join("storage/header_footer", outputID+".pdf")
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("mkdir: %w", err)
	}

	params := addheaderfooter.AddHeaderFooterParams{
		InputPath:  file.FilePath,
		OutputPath: outputPath,
		HeaderText: job.HeaderText,
		FooterText: job.FooterText,
		PageRange:  "", // pdfcpu's selectedPages: empty slice means every page
	}
	if err := addheaderfooter.AddHeaderFooterPDF(params); err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("addheaderfooter: %w", err)
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

	job.OutputFileID = &outputID
	job.Status = "done"
	return s.stg.AddHeaderFooter().Update(ctx, job)
}

func (s *addHeaderFooterService) markFailed(ctx context.Context, job *models.AddHeaderFooterJob) {
	job.Status = "failed"
	if err := s.stg.AddHeaderFooter().Update(ctx, job); err != nil {
		s.log.Error("header_footer: mark failed", logger.Error(err))
	}
}

func (s *addHeaderFooterService) GetByID(ctx context.Context, id string) (*models.AddHeaderFooterJob, error) {
	return s.stg.AddHeaderFooter().GetByID(ctx, id)
}
