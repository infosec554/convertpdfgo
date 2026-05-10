package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/detectblank"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/storage"
)

type DetectBlankService interface {
	Create(ctx context.Context, inputFileID string, userID string) (string, error)
	Process(ctx context.Context, jobID string) error
	GetByID(ctx context.Context, id string) (*models.DetectBlankPagesJob, error)
}

type detectBlankService struct {
	stg storage.IStorage
	log logger.ILogger
}

func NewDetectBlankService(stg storage.IStorage, log logger.ILogger) DetectBlankService {
	return &detectBlankService{stg: stg, log: log}
}

func (s *detectBlankService) Create(ctx context.Context, inputFileID string, userID string) (string, error) {
	job := &models.DetectBlankPagesJob{
		ID:          uuid.NewString(),
		UserID:      userID,
		InputFileID: inputFileID,
		Status:      "pending",
		CreatedAt:   time.Now(),
	}
	if err := s.stg.DetectBlankPages().Create(ctx, job); err != nil {
		return "", err
	}
	return job.ID, nil
}

func (s *detectBlankService) Process(ctx context.Context, jobID string) error {
	job, err := s.stg.DetectBlankPages().GetByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("get job: %w", err)
	}
	file, err := s.stg.File().GetByID(ctx, job.InputFileID)
	if err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("get input file: %w", err)
	}

	blankPages, err := detectblank.DetectBlankPages(file.FilePath, 10)
	if err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("detect blank: %w", err)
	}

	job.BlankPages = blankPages
	job.Status = "done"
	return s.stg.DetectBlankPages().Update(ctx, job)
}

func (s *detectBlankService) markFailed(ctx context.Context, job *models.DetectBlankPagesJob) {
	job.Status = "failed"
	if err := s.stg.DetectBlankPages().Update(ctx, job); err != nil {
		s.log.Error("detectblank: mark failed", logger.Error(err))
	}
}

func (s *detectBlankService) GetByID(ctx context.Context, id string) (*models.DetectBlankPagesJob, error) {
	return s.stg.DetectBlankPages().GetByID(ctx, id)
}
