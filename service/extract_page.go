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
	"convertpdfgo/storage"
)

type ExtractPageService interface {
	Create(ctx context.Context, req models.ExtractPagesRequest, userID *string) (string, error)
	Process(ctx context.Context, jobID string) error
	GetByID(ctx context.Context, id string) (*models.ExtractJob, error)
}

type extractPageService struct {
	stg storage.IStorage
	log logger.ILogger
}

func NewExtractService(stg storage.IStorage, log logger.ILogger) ExtractPageService {
	return &extractPageService{stg: stg, log: log}
}

func (s *extractPageService) Create(ctx context.Context, req models.ExtractPagesRequest, userID *string) (string, error) {
	job := &models.ExtractJob{
		ID:             uuid.NewString(),
		UserID:         userID,
		InputFileID:    req.InputFileID,
		PagesToExtract: req.PageRanges,
		Status:         "pending",
		CreatedAt:      time.Now(),
	}
	if err := s.stg.ExtractPage().Create(ctx, job); err != nil {
		return "", err
	}
	return job.ID, nil
}

func (s *extractPageService) Process(ctx context.Context, jobID string) error {
	job, err := s.stg.ExtractPage().GetByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("get job: %w", err)
	}
	file, err := s.stg.File().GetByID(ctx, job.InputFileID)
	if err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("get input file: %w", err)
	}

	pdfCtx, err := api.ReadContextFile(file.FilePath)
	if err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("read pdf context: %w", err)
	}
	totalPages := pdfCtx.PageCount

	outputDir := filepath.Join("storage/extract", job.ID)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("mkdir: %w", err)
	}

	var pages []string
	for _, r := range strings.Split(job.PagesToExtract, ",") {
		r = strings.TrimSpace(r)
		if strings.Contains(r, "-") {
			parts := strings.Split(r, "-")
			if len(parts) != 2 {
				continue
			}
			start, err1 := strconv.Atoi(parts[0])
			end, err2 := strconv.Atoi(parts[1])
			if err1 != nil || err2 != nil || start < 1 || end > totalPages || start > end {
				continue
			}
			for i := start; i <= end; i++ {
				pages = append(pages, strconv.Itoa(i))
			}
		} else {
			p, err := strconv.Atoi(r)
			if err != nil || p < 1 || p > totalPages {
				continue
			}
			pages = append(pages, strconv.Itoa(p))
		}
	}

	if len(pages) == 0 {
		s.markFailed(ctx, job)
		return fmt.Errorf("no valid pages")
	}

	// TrimFile produces a single PDF containing only the selected pages
	// (kept in original document order). Previously the code called
	// ExtractPagesFile, which emits one PDF per selected page — we then
	// only saved the first file and silently dropped the rest.
	outputPath := filepath.Join(outputDir, "extracted.pdf")
	if err := api.TrimFile(file.FilePath, outputPath, pages, relaxedPDFConf()); err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("pdfcpu trim: %w", err)
	}

	fi, err := os.Stat(outputPath)
	if err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("stat output: %w", err)
	}

	fileID, err := s.stg.File().Save(ctx, models.File{
		ID:         uuid.NewString(),
		UserID:     job.UserID,
		FileName:   "extracted.pdf",
		FilePath:   outputPath,
		FileType:   "application/pdf",
		FileSize:   fi.Size(),
		UploadedAt: time.Now(),
	})
	if err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("save output file: %w", err)
	}

	job.OutputFileID = &fileID
	job.Status = "done"
	return s.stg.ExtractPage().Update(ctx, job)
}

func (s *extractPageService) markFailed(ctx context.Context, job *models.ExtractJob) {
	job.Status = "failed"
	if err := s.stg.ExtractPage().Update(ctx, job); err != nil {
		s.log.Error("extract: mark failed", logger.Error(err))
	}
}

func (s *extractPageService) GetByID(ctx context.Context, id string) (*models.ExtractJob, error) {
	return s.stg.ExtractPage().GetByID(ctx, id)
}
