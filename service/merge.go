package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/storage"
)

type MergeService interface {
	Create(ctx context.Context, userID *string, inputFileIDs []string) (string, error)
	GetByID(ctx context.Context, id string) (*models.MergeJob, error)
	Process(ctx context.Context, jobID string) error
}

type mergeService struct {
	stg storage.IMergeStorage
	fs  storage.IFileStorage
	log logger.ILogger
}

func NewMergeService(stg storage.IStorage, log logger.ILogger) MergeService {
	return &mergeService{
		stg: stg.Merge(),
		fs:  stg.File(),
		log: log,
	}
}

func (s *mergeService) Create(ctx context.Context, userID *string, inputFileIDs []string) (string, error) {
	job := &models.MergeJob{
		ID:        uuid.New().String(),
		UserID:    userID,
		Status:    "pending",
		CreatedAt: time.Now(),
	}

	if err := s.stg.Create(ctx, job); err != nil {
		s.log.Error("merge: create job", logger.Error(err))
		return "", err
	}

	if err := s.stg.AddInputFiles(ctx, job.ID, inputFileIDs); err != nil {
		s.log.Error("merge: add input files", logger.Error(err))
		return "", err
	}
	return job.ID, nil
}

func (s *mergeService) GetByID(ctx context.Context, id string) (*models.MergeJob, error) {
	job, err := s.stg.GetByID(ctx, id)
	if err != nil {
		s.log.Error("merge: get by id", logger.Error(err))
		return nil, err
	}
	return job, nil
}

func (s *mergeService) Process(ctx context.Context, jobID string) error {
	job, err := s.stg.GetByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("get job: %w", err)
	}

	if len(job.InputFileIDs) < 2 {
		s.markFailed(ctx, job)
		return fmt.Errorf("need at least two files to merge")
	}

	if err := os.MkdirAll("storage/merge", 0o755); err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("mkdir: %w", err)
	}

	var inputPaths []string
	for _, fileID := range job.InputFileIDs {
		file, err := s.fs.GetByID(ctx, fileID)
		if err != nil {
			s.markFailed(ctx, job)
			return fmt.Errorf("get file %s: %w", fileID, err)
		}
		if _, err := os.Stat(file.FilePath); os.IsNotExist(err) {
			s.markFailed(ctx, job)
			return fmt.Errorf("input file missing: %s", file.FilePath)
		}
		inputPaths = append(inputPaths, file.FilePath)
	}

	outputID := uuid.NewString()
	outputPath := filepath.Join("storage/merge", outputID+".pdf")

	conf := model.NewDefaultConfiguration()
	if err := api.MergeCreateFile(inputPaths, outputPath, false, conf); err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("pdfcpu merge: %w", err)
	}

	info, err := os.Stat(outputPath)
	if err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("stat output: %w", err)
	}

	if _, err := s.fs.Save(ctx, models.File{
		ID:         outputID,
		UserID:     job.UserID,
		FileName:   filepath.Base(outputPath),
		FilePath:   outputPath,
		FileType:   "application/pdf",
		FileSize:   info.Size(),
		UploadedAt: time.Now(),
	}); err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("save output: %w", err)
	}

	job.OutputFileID = &outputID
	job.Status = "done"
	return s.stg.Update(ctx, job)
}

func (s *mergeService) markFailed(ctx context.Context, job *models.MergeJob) {
	job.Status = "failed"
	if err := s.stg.Update(ctx, job); err != nil {
		s.log.Error("merge: mark failed", logger.Error(err))
	}
}
