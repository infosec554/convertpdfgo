package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pdfcpu/pdfcpu/pkg/api"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/storage"
)

type SplitService interface {
	Create(ctx context.Context, req models.CreateSplitJobRequest, userID *string) (string, error)
	Process(ctx context.Context, jobID string) error
	GetByID(ctx context.Context, id string) (*models.SplitJob, error)
}

type splitService struct {
	stg storage.IStorage
	log logger.ILogger
}

func NewSplitService(stg storage.IStorage, log logger.ILogger) SplitService {
	return &splitService{stg: stg, log: log}
}

func (s *splitService) Create(ctx context.Context, req models.CreateSplitJobRequest, userID *string) (string, error) {
	job := &models.SplitJob{
		ID:            uuid.NewString(),
		UserID:        userID,
		InputFileID:   req.InputFileID,
		SplitRanges:   req.SplitRanges,
		OutputFileIDs: []string{},
		Status:        "pending",
		CreatedAt:     time.Now(),
	}
	if err := s.stg.Split().Create(ctx, job); err != nil {
		return "", err
	}
	return job.ID, nil
}

func (s *splitService) Process(ctx context.Context, jobID string) error {
	job, err := s.stg.Split().GetByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("get job: %w", err)
	}

	file, err := s.stg.File().GetByID(ctx, job.InputFileID)
	if err != nil {
		return fmt.Errorf("get input file: %w", err)
	}

	outputDir := fmt.Sprintf("storage/split/%s", job.ID)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	span := 1
	if v := strings.TrimSpace(job.SplitRanges); v != "" {
		if n, err := fmt.Sscanf(v, "%d", &span); err != nil || n != 1 || span < 1 {
			return fmt.Errorf("invalid split range: %s", v)
		}
	}

	if err := api.SplitFile(file.FilePath, outputDir, span, relaxedPDFConf()); err != nil {
		return fmt.Errorf("pdfcpu split: %w", err)
	}

	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return fmt.Errorf("read output dir: %w", err)
	}

	var outputIDs []string
	for _, f := range entries {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".pdf") {
			continue
		}
		fullPath := filepath.Join(outputDir, f.Name())
		fi, err := os.Stat(fullPath)
		if err != nil {
			continue
		}
		fileID, err := s.stg.File().Save(ctx, models.File{
			ID:         uuid.NewString(),
			UserID:     job.UserID,
			FileName:   f.Name(),
			FilePath:   fullPath,
			FileType:   "application/pdf",
			FileSize:   fi.Size(),
			UploadedAt: time.Now(),
		})
		if err != nil {
			continue
		}
		outputIDs = append(outputIDs, fileID)
	}

	if len(outputIDs) == 0 {
		return fmt.Errorf("no output files produced")
	}

	if err := s.stg.Split().UpdateOutputFiles(ctx, job.ID, outputIDs); err != nil {
		return fmt.Errorf("update output files: %w", err)
	}
	return nil
}

func (s *splitService) GetByID(ctx context.Context, id string) (*models.SplitJob, error) {
	return s.stg.Split().GetByID(ctx, id)
}
