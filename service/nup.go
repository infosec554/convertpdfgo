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

type NUpService interface {
	Create(ctx context.Context, req models.NUpRequest, userID *string) (string, error)
	Process(ctx context.Context, jobID string) error
	GetByID(ctx context.Context, id string) (*models.NUpJob, error)
}

type nupService struct {
	stg storage.IStorage
	log logger.ILogger
}

func NewNUpService(stg storage.IStorage, log logger.ILogger) NUpService {
	return &nupService{stg: stg, log: log}
}

func (s *nupService) Create(ctx context.Context, req models.NUpRequest, userID *string) (string, error) {
	switch req.N {
	case 2, 3, 4, 6, 8, 9, 12, 16:
	default:
		return "", fmt.Errorf("n must be 2,3,4,6,8,9,12,16")
	}
	orient := strings.ToLower(strings.TrimSpace(req.Orientation))
	if orient != "landscape" {
		orient = "portrait"
	}

	job := &models.NUpJob{
		ID:          uuid.NewString(),
		UserID:      userID,
		InputFileID: req.InputFileID,
		N:           req.N,
		Orientation: orient,
		Border:      req.Border,
		Status:      models.Pending,
		CreatedAt:   time.Now(),
	}
	if err := s.stg.NUp().Create(ctx, job); err != nil {
		return "", err
	}
	return job.ID, nil
}

func (s *nupService) Process(ctx context.Context, jobID string) error {
	job, err := s.stg.NUp().GetByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("get job: %w", err)
	}
	in, err := s.stg.File().GetByID(ctx, job.InputFileID)
	if err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("get input: %w", err)
	}

	outID := uuid.NewString()
	outDir := "storage/nup"
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("mkdir: %w", err)
	}
	outPath := filepath.Join(outDir, outID+".pdf")

	conf := relaxedPDFConf()
	descParts := []string{}
	if job.Border {
		descParts = append(descParts, "border:on")
	} else {
		descParts = append(descParts, "border:off")
	}
	if job.Orientation == "landscape" {
		// 'orient:rd' (right-down) gives a landscape feel for typical N-up;
		// pdfcpu's PDFNUpConfig accepts orient via the desc string.
		descParts = append(descParts, "orient:rd")
	}
	desc := strings.Join(descParts, ", ")

	nup, err := api.PDFNUpConfig(job.N, desc, conf)
	if err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("pdfcpu nup config: %w", err)
	}

	if err := api.NUpFile([]string{in.FilePath}, outPath, nil, nup, conf); err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("pdfcpu nup: %w", err)
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
	return s.stg.NUp().Update(ctx, job)
}

func (s *nupService) markFailed(ctx context.Context, job *models.NUpJob) {
	job.Status = models.Failed
	if err := s.stg.NUp().Update(ctx, job); err != nil {
		s.log.Error("nup: mark failed", logger.Error(err))
	}
}

func (s *nupService) GetByID(ctx context.Context, id string) (*models.NUpJob, error) {
	return s.stg.NUp().GetByID(ctx, id)
}
