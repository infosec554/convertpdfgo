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

type MetadataService interface {
	Create(ctx context.Context, req models.MetadataRequest, userID *string) (string, error)
	Process(ctx context.Context, jobID string) error
	GetByID(ctx context.Context, id string) (*models.MetadataJob, error)
}

type metadataService struct {
	stg storage.IStorage
	log logger.ILogger
}

func NewMetadataService(stg storage.IStorage, log logger.ILogger) MetadataService {
	return &metadataService{stg: stg, log: log}
}

func (s *metadataService) Create(ctx context.Context, req models.MetadataRequest, userID *string) (string, error) {
	job := &models.MetadataJob{
		ID:          uuid.NewString(),
		UserID:      userID,
		InputFileID: req.InputFileID,
		Title:       req.Title,
		Author:      req.Author,
		Subject:     req.Subject,
		Creator:     req.Creator,
		Status:      models.Pending,
		CreatedAt:   time.Now(),
	}
	if len(req.Keywords) > 0 {
		joined := strings.Join(req.Keywords, ", ")
		job.Keywords = &joined
	}
	if err := s.stg.Metadata().Create(ctx, job); err != nil {
		return "", err
	}
	return job.ID, nil
}

func (s *metadataService) Process(ctx context.Context, jobID string) error {
	job, err := s.stg.Metadata().GetByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("get job: %w", err)
	}
	in, err := s.stg.File().GetByID(ctx, job.InputFileID)
	if err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("get input: %w", err)
	}

	outID := uuid.NewString()
	outDir := "storage/metadata"
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("mkdir: %w", err)
	}
	outPath := filepath.Join(outDir, outID+".pdf")

	props := map[string]string{}
	addIfSet(props, "Title", job.Title)
	addIfSet(props, "Author", job.Author)
	addIfSet(props, "Subject", job.Subject)
	addIfSet(props, "Keywords", job.Keywords)
	addIfSet(props, "Creator", job.Creator)
	if len(props) == 0 {
		s.markFailed(ctx, job)
		return fmt.Errorf("no metadata fields provided")
	}

	conf := relaxedPDFConf()
	if err := api.AddPropertiesFile(in.FilePath, outPath, props, conf); err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("pdfcpu metadata: %w", err)
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
	return s.stg.Metadata().Update(ctx, job)
}

func (s *metadataService) markFailed(ctx context.Context, job *models.MetadataJob) {
	job.Status = models.Failed
	if err := s.stg.Metadata().Update(ctx, job); err != nil {
		s.log.Error("metadata: mark failed", logger.Error(err))
	}
}

func (s *metadataService) GetByID(ctx context.Context, id string) (*models.MetadataJob, error) {
	return s.stg.Metadata().GetByID(ctx, id)
}

func addIfSet(m map[string]string, key string, v *string) {
	if v == nil {
		return
	}
	if s := strings.TrimSpace(*v); s != "" {
		m[key] = s
	}
}
