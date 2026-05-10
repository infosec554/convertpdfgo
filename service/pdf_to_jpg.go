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
	createzipfromfiles "convertpdfgo/pkg/createZipFromFiles"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/storage"
)

type PDFToJPGService interface {
	Create(ctx context.Context, req models.PDFToJPGRequest, userID *string) (string, error)
	Process(ctx context.Context, jobID string) error
	GetByID(ctx context.Context, id string) (*models.PDFToJPGJob, error)
}

type pdfToJPGService struct {
	stg storage.IStorage
	log logger.ILogger
}

func NewPDFToJPGService(stg storage.IStorage, log logger.ILogger) PDFToJPGService {
	return &pdfToJPGService{stg: stg, log: log}
}

func (s *pdfToJPGService) Create(ctx context.Context, req models.PDFToJPGRequest, userID *string) (string, error) {
	job := &models.PDFToJPGJob{
		ID:            uuid.NewString(),
		UserID:        userID,
		InputFileID:   req.InputFileID,
		OutputFileIDs: []string{},
		Status:        "pending",
		CreatedAt:     time.Now(),
	}
	if err := s.stg.PDFToJPG().Create(ctx, job); err != nil {
		return "", err
	}
	return job.ID, nil
}

func (s *pdfToJPGService) Process(ctx context.Context, jobID string) error {
	job, err := s.stg.PDFToJPG().GetByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("get job: %w", err)
	}
	file, err := s.stg.File().GetByID(ctx, job.InputFileID)
	if err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("get input file: %w", err)
	}

	outputDir := filepath.Join("storage/pdf_to_jpg", job.ID)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("mkdir: %w", err)
	}

	outputPath := filepath.Join(outputDir, "page")
	cmd := exec.CommandContext(ctx, "pdftoppm", "-jpeg", file.FilePath, outputPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("pdftoppm: %w (%s)", err, string(out))
	}

	var jpgPaths []string
	if err := filepath.Walk(outputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if filepath.Ext(path) == ".jpg" {
			jpgPaths = append(jpgPaths, path)
			fileID := uuid.NewString()
			if _, e := s.stg.File().Save(ctx, models.File{
				ID:         fileID,
				UserID:     job.UserID,
				FileName:   filepath.Base(path),
				FilePath:   path,
				FileType:   "image/jpeg",
				FileSize:   info.Size(),
				UploadedAt: time.Now(),
			}); e != nil {
				return e
			}
			job.OutputFileIDs = append(job.OutputFileIDs, fileID)
		}
		return nil
	}); err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("collect jpg files: %w", err)
	}

	zipPath := filepath.Join("storage/pdf_to_jpg", job.ID+".zip")
	zipFile, err := os.Create(zipPath)
	if err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("create zip: %w", err)
	}
	if err := createzipfromfiles.CreateZipFromFiles(zipFile, jpgPaths); err != nil {
		zipFile.Close()
		s.markFailed(ctx, job)
		return fmt.Errorf("write zip: %w", err)
	}
	zipFile.Close()

	zi, err := os.Stat(zipPath)
	if err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("stat zip: %w", err)
	}

	zipID := uuid.NewString()
	if _, err := s.stg.File().Save(ctx, models.File{
		ID:         zipID,
		UserID:     job.UserID,
		FileName:   filepath.Base(zipPath),
		FilePath:   zipPath,
		FileType:   "application/zip",
		FileSize:   zi.Size(),
		UploadedAt: time.Now(),
	}); err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("save zip: %w", err)
	}

	job.ZipFileID = &zipID
	job.Status = "done"
	return s.stg.PDFToJPG().Update(ctx, job)
}

func (s *pdfToJPGService) markFailed(ctx context.Context, job *models.PDFToJPGJob) {
	job.Status = "failed"
	if err := s.stg.PDFToJPG().Update(ctx, job); err != nil {
		s.log.Error("pdf_to_jpg: mark failed", logger.Error(err))
	}
}

func (s *pdfToJPGService) GetByID(ctx context.Context, id string) (*models.PDFToJPGJob, error) {
	return s.stg.PDFToJPG().GetByID(ctx, id)
}
