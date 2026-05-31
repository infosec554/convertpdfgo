package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pdfcpu/pdfcpu/pkg/api"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
	pkg "convertpdfgo/pkg/string_to_int"
	"convertpdfgo/storage"
)

type RemovePageService interface {
	Create(ctx context.Context, req models.RemovePagesRequest, userID *string) (string, error)
	Process(ctx context.Context, jobID string) error
	GetByID(ctx context.Context, id string) (*models.RemoveJob, error)
}

type removePageService struct {
	stg storage.IStorage
	log logger.ILogger
}

func NewRemoveService(stg storage.IStorage, log logger.ILogger) RemovePageService {
	return &removePageService{stg: stg, log: log}
}

func (s *removePageService) Create(ctx context.Context, req models.RemovePagesRequest, userID *string) (string, error) {
	job := &models.RemoveJob{
		ID:            uuid.NewString(),
		UserID:        userID,
		InputFileID:   req.InputFileID,
		PagesToRemove: req.PagesToRemove,
		Status:        "pending",
		CreatedAt:     time.Now(),
	}
	if err := s.stg.RemovePage().Create(ctx, job); err != nil {
		return "", err
	}
	return job.ID, nil
}

func (s *removePageService) Process(ctx context.Context, jobID string) error {
	job, err := s.stg.RemovePage().GetByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("get job: %w", err)
	}
	file, err := s.stg.File().GetByID(ctx, job.InputFileID)
	if err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("get input file: %w", err)
	}

	outputDir := "storage/remove"
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("mkdir: %w", err)
	}
	outputID := uuid.NewString()
	outputPath := filepath.Join(outputDir, outputID+".pdf")

	pageList, err := parsePageList(job.PagesToRemove)
	if err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("parse pages: %w", err)
	}
	pageStrs := pkg.IntSliceToStringSlice(pageList)

	if err := api.RemovePagesFile(file.FilePath, outputPath, pageStrs, relaxedPDFConf()); err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("pdfcpu remove: %w", err)
	}

	info, err := os.Stat(outputPath)
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
		FileSize:   info.Size(),
		UploadedAt: time.Now(),
	}); err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("save output: %w", err)
	}

	job.OutputFileID = &outputID
	job.Status = "done"
	return s.stg.RemovePage().Update(ctx, job)
}

func (s *removePageService) markFailed(ctx context.Context, job *models.RemoveJob) {
	job.Status = "failed"
	if err := s.stg.RemovePage().Update(ctx, job); err != nil {
		s.log.Error("removepage: mark failed", logger.Error(err))
	}
}

func (s *removePageService) GetByID(ctx context.Context, id string) (*models.RemoveJob, error) {
	return s.stg.RemovePage().GetByID(ctx, id)
}

func parsePageList(input string) ([]int, error) {
	var pages []int
	for _, token := range strings.Split(input, ",") {
		token = strings.TrimSpace(token)
		if strings.Contains(token, "-") {
			parts := strings.Split(token, "-")
			if len(parts) != 2 {
				return nil, fmt.Errorf("invalid range: %s", token)
			}
			start, err1 := strconv.Atoi(parts[0])
			end, err2 := strconv.Atoi(parts[1])
			if err1 != nil || err2 != nil || start > end {
				return nil, fmt.Errorf("invalid range values: %s", token)
			}
			for i := start; i <= end; i++ {
				pages = append(pages, i)
			}
		} else {
			p, err := strconv.Atoi(token)
			if err != nil {
				return nil, fmt.Errorf("invalid page number: %s", token)
			}
			pages = append(pages, p)
		}
	}
	return pages, nil
}
