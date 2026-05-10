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

type ExtractTextService interface {
	Create(ctx context.Context, req models.ExtractTextRequest, userID *string) (string, error)
	Process(ctx context.Context, jobID string) error
	GetByID(ctx context.Context, id string) (*models.ExtractTextJob, error)
}

type extractTextService struct {
	stg storage.IStorage
	log logger.ILogger
}

func NewExtractTextService(stg storage.IStorage, log logger.ILogger) ExtractTextService {
	return &extractTextService{stg: stg, log: log}
}

func (s *extractTextService) Create(ctx context.Context, req models.ExtractTextRequest, userID *string) (string, error) {
	format := req.Format
	if format != "json" {
		format = "txt"
	}
	job := &models.ExtractTextJob{
		ID:          uuid.NewString(),
		UserID:      userID,
		InputFileID: req.InputFileID,
		Format:      format,
		Status:      models.Pending,
		CreatedAt:   time.Now(),
	}
	if err := s.stg.ExtractText().Create(ctx, job); err != nil {
		return "", err
	}
	return job.ID, nil
}

func (s *extractTextService) Process(ctx context.Context, jobID string) error {
	job, err := s.stg.ExtractText().GetByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("get job: %w", err)
	}
	in, err := s.stg.File().GetByID(ctx, job.InputFileID)
	if err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("get input: %w", err)
	}

	outID := uuid.NewString()
	outDir := "storage/extract_text"
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("mkdir: %w", err)
	}

	ext := ".txt"
	mime := "text/plain"
	if job.Format == "json" {
		ext = ".json"
		mime = "application/json"
	}
	outPath := filepath.Join(outDir, outID+ext)

	if err := runPdftotext(ctx, in.FilePath, outPath, job.Format); err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("pdftotext: %w", err)
	}

	fi, err := os.Stat(outPath)
	if err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("stat output: %w", err)
	}

	if _, err := s.stg.File().Save(ctx, models.File{
		ID:         outID,
		UserID:     job.UserID,
		FileName:   filepath.Base(outPath),
		FilePath:   outPath,
		FileType:   mime,
		FileSize:   fi.Size(),
		UploadedAt: time.Now(),
	}); err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("save output: %w", err)
	}

	job.OutputFileID = &outID
	job.Status = models.Done
	return s.stg.ExtractText().Update(ctx, job)
}

// runPdftotext shells out to poppler. For JSON we wrap each page (delimited
// by 0x0c form feed in the txt output) into a per-page array.
func runPdftotext(ctx context.Context, in, out, format string) error {
	if format == "json" {
		// Get plain text first; convert to JSON afterwards.
		raw := out + ".raw"
		defer os.Remove(raw)
		cmd := exec.CommandContext(ctx, "pdftotext", "-layout", in, raw)
		if b, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("%w: %s", err, string(b))
		}
		data, err := os.ReadFile(raw)
		if err != nil {
			return err
		}
		return writePagesJSON(out, data)
	}
	cmd := exec.CommandContext(ctx, "pdftotext", "-layout", in, out)
	if b, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %s", err, string(b))
	}
	return nil
}

func (s *extractTextService) markFailed(ctx context.Context, job *models.ExtractTextJob) {
	job.Status = models.Failed
	if err := s.stg.ExtractText().Update(ctx, job); err != nil {
		s.log.Error("extract_text: mark failed", logger.Error(err))
	}
}

func (s *extractTextService) GetByID(ctx context.Context, id string) (*models.ExtractTextJob, error) {
	return s.stg.ExtractText().GetByID(ctx, id)
}
