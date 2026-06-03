package service

import (
	"context"
	"fmt"
	"image"
	_ "image/jpeg" // register decoders for image.DecodeConfig
	_ "image/png"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/pdfcpu/pdfcpu/pkg/api"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/storage"
)

type SignService interface {
	Create(ctx context.Context, req models.SignRequest, userID *string) (string, error)
	Process(ctx context.Context, jobID string) error
	GetByID(ctx context.Context, id string) (*models.SignJob, error)
}

type signService struct {
	stg storage.IStorage
	log logger.ILogger
}

func NewSignService(stg storage.IStorage, log logger.ILogger) SignService {
	return &signService{stg: stg, log: log}
}

func (s *signService) Create(ctx context.Context, req models.SignRequest, userID *string) (string, error) {
	job := &models.SignJob{
		ID:              uuid.NewString(),
		UserID:          userID,
		InputFileID:     req.InputFileID,
		SignatureFileID: req.SignatureFileID,
		Page:            req.Page,
		X:               req.X,
		Y:               req.Y,
		Width:           req.Width,
		Status:          models.Pending,
		CreatedAt:       time.Now(),
	}
	if err := s.stg.Sign().Create(ctx, job); err != nil {
		return "", err
	}
	return job.ID, nil
}

func (s *signService) Process(ctx context.Context, jobID string) error {
	job, err := s.stg.Sign().GetByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("get job: %w", err)
	}

	in, err := s.stg.File().GetByID(ctx, job.InputFileID)
	if err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("get input file: %w", err)
	}
	sig, err := s.stg.File().GetByID(ctx, job.SignatureFileID)
	if err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("get signature file: %w", err)
	}

	outID := uuid.NewString()
	outDir := "storage/sign"
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("mkdir: %w", err)
	}
	outPath := filepath.Join(outDir, outID+".pdf")

	// pdfcpu's descriptor takes a scale FACTOR, not an absolute width,
	// so we read the signature image's intrinsic width and turn the
	// user's desired width-in-points into a factor that pdfcpu can use.
	sigFile, err := os.Open(sig.FilePath)
	if err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("open signature: %w", err)
	}
	cfg, _, err := image.DecodeConfig(sigFile)
	sigFile.Close()
	if err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("decode signature: %w", err)
	}
	if cfg.Width <= 0 {
		s.markFailed(ctx, job)
		return fmt.Errorf("signature has zero width")
	}
	scaleFactor := job.Width / float64(cfg.Width)

	// pdfcpu watermark descriptor:
	//   pos:bl              anchor at bottom-left of the page (PDF origin)
	//   off:X Y             offset in points from that anchor
	//   scalefactor:F abs   absolute scale of the source image
	//   rot:0               no rotation
	//   op:1.0              fully opaque (it's a signature, not a watermark)
	// onTop=true so the signature renders above page content.
	desc := fmt.Sprintf(
		"pos:bl, off:%.1f %.1f, scalefactor:%.4f abs, rot:0, op:1.0",
		job.X, job.Y, scaleFactor,
	)
	pages := []string{strconv.Itoa(job.Page)}
	conf := relaxedPDFConf()

	if err := api.AddImageWatermarksFile(in.FilePath, outPath, pages, true, sig.FilePath, desc, conf); err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("pdfcpu sign: %w", err)
	}

	fi, err := os.Stat(outPath)
	if err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("stat output: %w", err)
	}

	if _, err := s.stg.File().Save(ctx, models.File{
		ID:         outID,
		UserID:     job.UserID,
		FileName:   "signed-" + filepath.Base(in.FilePath),
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
	return s.stg.Sign().Update(ctx, job)
}

func (s *signService) markFailed(ctx context.Context, job *models.SignJob) {
	job.Status = models.Failed
	if err := s.stg.Sign().Update(ctx, job); err != nil {
		s.log.Error("sign: mark failed", logger.Error(err))
	}
}

func (s *signService) GetByID(ctx context.Context, id string) (*models.SignJob, error) {
	return s.stg.Sign().GetByID(ctx, id)
}
