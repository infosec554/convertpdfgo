package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/skip2/go-qrcode"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/storage"
)

type QRCodeService interface {
	Create(ctx context.Context, req models.CreateQRCodeRequest, userID *string) (string, error)
	Process(ctx context.Context, jobID string) error
	GetByID(ctx context.Context, id string) (*models.QRCodeJob, error)
}

type qrCodeService struct {
	stg storage.IStorage
	log logger.ILogger
}

func NewQRCodeService(stg storage.IStorage, log logger.ILogger) QRCodeService {
	return &qrCodeService{stg: stg, log: log}
}

func (s *qrCodeService) Create(ctx context.Context, req models.CreateQRCodeRequest, userID *string) (string, error) {
	job := &models.QRCodeJob{
		ID:          uuid.NewString(),
		UserID:      userID,
		InputFileID: req.InputFileID,
		QRContent:   req.QRContent,
		Position:    req.Position,
		Size:        req.Size,
		Status:      "pending",
		CreatedAt:   time.Now(),
	}
	if err := s.stg.QRCode().Create(ctx, job); err != nil {
		return "", err
	}
	return job.ID, nil
}

func (s *qrCodeService) Process(ctx context.Context, jobID string) error {
	job, err := s.stg.QRCode().GetByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("get job: %w", err)
	}
	file, err := s.stg.File().GetByID(ctx, job.InputFileID)
	if err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("get input file: %w", err)
	}

	if err := os.MkdirAll("storage/qr_code", 0o755); err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("mkdir qr: %w", err)
	}
	qrPngPath := filepath.Join("storage/qr_code", job.ID+".png")
	if err := qrcode.WriteFile(job.QRContent, qrcode.Medium, job.Size, qrPngPath); err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("generate qr: %w", err)
	}

	if err := os.MkdirAll("storage/qr_code_pdf", 0o755); err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("mkdir qr_pdf: %w", err)
	}
	outputID := uuid.NewString()
	outputPath := filepath.Join("storage/qr_code_pdf", outputID+".pdf")

	conf := model.NewDefaultConfiguration()
	pos := mapQRPosition(job.Position)
	wmDetails := fmt.Sprintf("pos:%s, scale:1 rel, rot:0", pos)

	wm, err := pdfcpu.ParseImageWatermarkDetails(qrPngPath, wmDetails, true, conf.Unit)
	if err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("parse watermark: %w", err)
	}
	if err := api.AddWatermarksFile(file.FilePath, outputPath, nil, wm, conf); err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("add watermark: %w", err)
	}

	fi, err := os.Stat(outputPath)
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
		FileSize:   fi.Size(),
		UploadedAt: time.Now(),
	}); err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("save output: %w", err)
	}

	job.OutputFileID = &outputID
	job.Status = "done"
	return s.stg.QRCode().Update(ctx, job)
}

func (s *qrCodeService) markFailed(ctx context.Context, job *models.QRCodeJob) {
	job.Status = "failed"
	if err := s.stg.QRCode().Update(ctx, job); err != nil {
		s.log.Error("qr_code: mark failed", logger.Error(err))
	}
}

func (s *qrCodeService) GetByID(ctx context.Context, id string) (*models.QRCodeJob, error) {
	return s.stg.QRCode().GetByID(ctx, id)
}

func mapQRPosition(position string) string {
	switch position {
	case "top-left":
		return "tl"
	case "top-right":
		return "tr"
	case "bottom-left":
		return "bl"
	case "bottom-right":
		return "br"
	default:
		return "c"
	}
}
