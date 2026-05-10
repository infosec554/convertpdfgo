package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/pdfcpu/pdfcpu/pkg/api"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/storage"
)

type InspectService interface {
	Create(ctx context.Context, fileID, userID string) (string, error)
	GetByID(ctx context.Context, id string) (*models.InspectJob, error)
}

type inspectService struct {
	stg storage.IStorage
	log logger.ILogger
}

func NewInspectService(stg storage.IStorage, log logger.ILogger) InspectService {
	return &inspectService{
		stg: stg,
		log: log,
	}
}

func (s *inspectService) Create(ctx context.Context, fileID, userID string) (string, error) {
	s.log.Info("InspectService.Create called")

	// 1. Faylni olish
	file, err := s.stg.File().GetByID(ctx, fileID)
	if err != nil {
		s.log.Error("failed to get input file", logger.Error(err))
		return "", err
	}

	jobID := uuid.NewString()
	pdfPath := file.FilePath

	// 3. Context ni o‘qish
	ctxPDF, err := api.ReadContextFile(pdfPath)
	if err != nil {
		s.log.Error("failed to read PDF context", logger.Error(err))
		return "", err
	}

	// 4. Job ma’lumotlarini yaratish
	job := &models.InspectJob{
		ID:        jobID,
		UserID:    userID,
		FileID:    fileID,
		PageCount: ctxPDF.PageCount,
		Title:     ctxPDF.Title,
		Author:    ctxPDF.Author,
		Subject:   ctxPDF.Subject,
		Keywords:  ctxPDF.Keywords,
		Status:    "done",
		CreatedAt: time.Now(),
	}

	// 5. Bazaga yozish
	if err := s.stg.Inspect().Create(ctx, job); err != nil {
		s.log.Error("failed to save inspect job", logger.Error(err))
		return "", err
	}

	s.log.Info("PDF inspect completed", logger.String("jobID", jobID))
	return jobID, nil
}

func (s *inspectService) GetByID(ctx context.Context, id string) (*models.InspectJob, error) {
	job, err := s.stg.Inspect().GetByID(ctx, id)
	if err != nil {
		s.log.Error("failed to get inspect job", logger.Error(err))
		return nil, err
	}
	return job, nil
}
