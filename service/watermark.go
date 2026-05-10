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
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/storage"
)

type WatermarkService interface {
	Create(ctx context.Context, req models.WatermarkRequest, userID *string) (string, error)
	Process(ctx context.Context, jobID string) error
	GetByID(ctx context.Context, id string) (*models.WatermarkJob, error)
}

type watermarkService struct {
	stg storage.IStorage
	log logger.ILogger
}

func NewWatermarkService(stg storage.IStorage, log logger.ILogger) WatermarkService {
	return &watermarkService{stg: stg, log: log}
}

func (s *watermarkService) Create(ctx context.Context, req models.WatermarkRequest, userID *string) (string, error) {
	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	if mode != "text" && mode != "image" {
		return "", fmt.Errorf("mode must be text or image")
	}
	if mode == "text" && strings.TrimSpace(req.Text) == "" {
		return "", fmt.Errorf("text required for text watermark")
	}
	if mode == "image" && req.ImageFileID == "" {
		return "", fmt.Errorf("image_file_id required for image watermark")
	}

	op := req.Opacity
	if op <= 0 || op > 1 {
		op = 0.4
	}
	pos := req.Position
	if pos == "" {
		pos = "c"
	}
	pages := strings.TrimSpace(req.Pages)
	if pages == "" {
		pages = "all"
	}

	job := &models.WatermarkJob{
		ID:          uuid.NewString(),
		UserID:      userID,
		InputFileID: req.InputFileID,
		Mode:        mode,
		Text:        req.Text,
		Position:    pos,
		Rotation:    req.Rotation,
		Opacity:     op,
		Pages:       pages,
		Status:      models.Pending,
		CreatedAt:   time.Now(),
	}
	if mode == "image" {
		id := req.ImageFileID
		job.ImageFileID = &id
	}
	if err := s.stg.Watermark().Create(ctx, job); err != nil {
		return "", err
	}
	return job.ID, nil
}

func (s *watermarkService) Process(ctx context.Context, jobID string) error {
	job, err := s.stg.Watermark().GetByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("get job: %w", err)
	}

	in, err := s.stg.File().GetByID(ctx, job.InputFileID)
	if err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("get input file: %w", err)
	}

	outID := uuid.NewString()
	outDir := "storage/watermark"
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("mkdir: %w", err)
	}
	outPath := filepath.Join(outDir, outID+".pdf")

	desc := fmt.Sprintf("scale:1 abs, rot:%d, op:%.2f, pos:%s", job.Rotation, job.Opacity, job.Position)
	pages := pageSelection(job.Pages)
	conf := model.NewDefaultConfiguration()

	switch job.Mode {
	case "text":
		if err := api.AddTextWatermarksFile(in.FilePath, outPath, pages, true, job.Text, desc, conf); err != nil {
			s.markFailed(ctx, job)
			return fmt.Errorf("pdfcpu watermark text: %w", err)
		}
	case "image":
		if job.ImageFileID == nil {
			s.markFailed(ctx, job)
			return fmt.Errorf("missing image_file_id")
		}
		img, err := s.stg.File().GetByID(ctx, *job.ImageFileID)
		if err != nil {
			s.markFailed(ctx, job)
			return fmt.Errorf("get image file: %w", err)
		}
		if err := api.AddImageWatermarksFile(in.FilePath, outPath, pages, true, img.FilePath, desc, conf); err != nil {
			s.markFailed(ctx, job)
			return fmt.Errorf("pdfcpu watermark image: %w", err)
		}
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
		FileType:   "application/pdf",
		FileSize:   fi.Size(),
		UploadedAt: time.Now(),
	}); err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("save output: %w", err)
	}

	job.OutputFileID = &outID
	job.Status = models.Done
	return s.stg.Watermark().Update(ctx, job)
}

func (s *watermarkService) markFailed(ctx context.Context, job *models.WatermarkJob) {
	job.Status = models.Failed
	if err := s.stg.Watermark().Update(ctx, job); err != nil {
		s.log.Error("watermark: mark failed", logger.Error(err))
	}
}

func (s *watermarkService) GetByID(ctx context.Context, id string) (*models.WatermarkJob, error) {
	return s.stg.Watermark().GetByID(ctx, id)
}

// pageSelection turns a user-facing page string into pdfcpu's []string form.
// "all" or "" → nil (every page); "1-3,5" passes through.
func pageSelection(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" || strings.EqualFold(s, "all") {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
