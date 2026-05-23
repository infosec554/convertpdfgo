package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/pdfcpu/pdfcpu/pkg/api"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/storage"
)

// ErrInvalidPDF signals that pdfcpu refused to parse the input bytes — a
// caller-side problem (corrupt or non-PDF upload), not a server fault.
// Handlers should translate it to 422 instead of 500.
var ErrInvalidPDF = errors.New("invalid or malformed PDF")

type InspectService interface {
	// Create runs the inspect synchronously inside Create (no enqueue),
	// since pdfcpu's ReadContextFile is cheap. Matches the rest of the
	// service Create signatures even though there's no async hand-off.
	Create(ctx context.Context, req models.InspectRequest, userID *string) (string, error)
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

func (s *inspectService) Create(ctx context.Context, req models.InspectRequest, userID *string) (string, error) {
	file, err := s.stg.File().GetByID(ctx, req.FileID)
	if err != nil {
		s.log.Error("inspect: get input file", logger.Error(err))
		return "", err
	}

	ctxPDF, err := api.ReadContextFile(file.FilePath)
	if err != nil {
		// pdfcpu surfaces parse failures via a string that always
		// includes "pdfcpu:" — wrap them in ErrInvalidPDF so the
		// handler can return 422 instead of leaking a stack-trace 500
		// for what is fundamentally a bad upload.
		s.log.Warning("inspect: pdfcpu rejected input", logger.Error(err))
		return "", fmt.Errorf("%w: %s", ErrInvalidPDF, err.Error())
	}

	job := &models.InspectJob{
		ID:        uuid.NewString(),
		UserID:    userID,
		FileID:    req.FileID,
		PageCount: ctxPDF.PageCount,
		Title:     ctxPDF.Title,
		Author:    ctxPDF.Author,
		Subject:   ctxPDF.Subject,
		Keywords:  ctxPDF.Keywords,
		Status:    "done",
		CreatedAt: time.Now(),
	}
	if err := s.stg.Inspect().Create(ctx, job); err != nil {
		s.log.Error("inspect: save job", logger.Error(err))
		return "", err
	}
	return job.ID, nil
}

func (s *inspectService) GetByID(ctx context.Context, id string) (*models.InspectJob, error) {
	job, err := s.stg.Inspect().GetByID(ctx, id)
	if err != nil {
		s.log.Error("failed to get inspect job", logger.Error(err))
		return nil, err
	}
	return job, nil
}
