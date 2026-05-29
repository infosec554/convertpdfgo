package service

import (
	"bytes"
	"context"
	"fmt"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "image/gif"

	"github.com/google/uuid"
	"github.com/jung-kurt/gofpdf"
	"golang.org/x/image/bmp"
	"golang.org/x/image/tiff"
	"golang.org/x/image/webp"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/storage"
)

type JPGToPDFService interface {
	CreateJob(ctx context.Context, userID *string, inputFileIDs []string) (string, error)
	Process(ctx context.Context, jobID string) error
	GetJobByID(ctx context.Context, id string) (*models.JPGToPDFJob, error)
}

type jpgToPDFService struct {
	stg storage.IStorage
	log logger.ILogger
}

func NewJPGToPDFService(stg storage.IStorage, log logger.ILogger) JPGToPDFService {
	return &jpgToPDFService{stg: stg, log: log}
}

func (s *jpgToPDFService) CreateJob(ctx context.Context, userID *string, inputFileIDs []string) (string, error) {
	if len(inputFileIDs) == 0 {
		return "", fmt.Errorf("no input files provided")
	}
	job := &models.JPGToPDFJob{
		ID:           uuid.NewString(),
		UserID:       userID,
		InputFileIDs: inputFileIDs,
		Status:       "pending",
		CreatedAt:    time.Now(),
	}
	if err := s.stg.JPGToPDF().Create(ctx, job); err != nil {
		return "", err
	}
	return job.ID, nil
}

func (s *jpgToPDFService) Process(ctx context.Context, jobID string) error {
	job, err := s.stg.JPGToPDF().GetByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("get job: %w", err)
	}

	var inputPaths []string
	for _, fileID := range job.InputFileIDs {
		file, err := s.stg.File().GetByID(ctx, fileID)
		if err != nil {
			s.markFailed(ctx, job)
			return fmt.Errorf("input file %s: %w", fileID, err)
		}
		inputPaths = append(inputPaths, file.FilePath)
	}

	outputID := uuid.NewString()
	outputDir := filepath.Join("storage", "jpg_to_pdf")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("mkdir: %w", err)
	}
	outputPath := filepath.Join(outputDir, outputID+".pdf")

	pdf := gofpdf.New("P", "mm", "A4", "")
	for i, imgPath := range inputPaths {
		usePath, imgType, cleanup, err := normaliseImage(imgPath)
		if err != nil {
			s.markFailed(ctx, job)
			return fmt.Errorf("input %d (%s): %w", i+1, filepath.Base(imgPath), err)
		}
		pdf.AddPage()
		pdf.ImageOptions(usePath, 10, 10, 190, 0, false, gofpdf.ImageOptions{ImageType: imgType, ReadDpi: true}, 0, "")
		cleanup()
	}
	if err := pdf.OutputFileAndClose(outputPath); err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("gofpdf: %w", err)
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

	if err := s.stg.JPGToPDF().UpdateStatusAndOutput(ctx, job.ID, "done", outputID); err != nil {
		return fmt.Errorf("update job: %w", err)
	}
	return nil
}

func (s *jpgToPDFService) markFailed(ctx context.Context, job *models.JPGToPDFJob) {
	if err := s.stg.JPGToPDF().UpdateStatusAndOutput(ctx, job.ID, "failed", ""); err != nil {
		s.log.Error("jpgtopdf: mark failed", logger.Error(err))
	}
}

func (s *jpgToPDFService) GetJobByID(ctx context.Context, id string) (*models.JPGToPDFJob, error) {
	return s.stg.JPGToPDF().GetByID(ctx, id)
}

// normaliseImage gives gofpdf a path it can decode. gofpdf supports JPG,
// PNG, and GIF natively; we detect via magic bytes (extensions are
// unreliable — most screenshots are PNG even when named .jpg) and for
// WebP / TIFF / BMP we transparently re-encode to JPEG so the worker
// doesn't fail on perfectly valid images.
func normaliseImage(path string) (outPath, imgType string, cleanup func(), err error) {
	cleanup = func() {}
	f, err := os.Open(path)
	if err != nil {
		return "", "", cleanup, fmt.Errorf("open: %w", err)
	}
	defer f.Close()

	head := make([]byte, 16)
	n, _ := io.ReadFull(f, head)
	head = head[:n]
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return "", "", cleanup, fmt.Errorf("seek: %w", err)
	}

	switch {
	case len(head) >= 3 && head[0] == 0xFF && head[1] == 0xD8 && head[2] == 0xFF:
		return path, "JPG", cleanup, nil
	case len(head) >= 8 && head[0] == 0x89 && head[1] == 0x50 && head[2] == 0x4E && head[3] == 0x47:
		return path, "PNG", cleanup, nil
	case len(head) >= 4 && head[0] == 0x47 && head[1] == 0x49 && head[2] == 0x46:
		return path, "GIF", cleanup, nil
	}

	// Re-encode WebP / TIFF / BMP to JPEG so gofpdf can take it.
	tmp, err := os.CreateTemp("", "jpgtopdf-*.jpg")
	if err != nil {
		return "", "", cleanup, fmt.Errorf("temp file: %w", err)
	}
	cleanup = func() { tmp.Close(); os.Remove(tmp.Name()) }

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, f); err != nil {
		cleanup()
		return "", "", func() {}, fmt.Errorf("read: %w", err)
	}
	lower := strings.ToLower(filepath.Ext(path))
	switch {
	case len(head) >= 12 && string(head[0:4]) == "RIFF" && string(head[8:12]) == "WEBP":
		img, derr := webp.Decode(&buf)
		if derr != nil {
			cleanup()
			return "", "", func() {}, fmt.Errorf("webp decode: %w", derr)
		}
		if err := jpeg.Encode(tmp, img, &jpeg.Options{Quality: 92}); err != nil {
			cleanup()
			return "", "", func() {}, fmt.Errorf("jpeg encode: %w", err)
		}
	case len(head) >= 2 && head[0] == 0x42 && head[1] == 0x4D:
		img, derr := bmp.Decode(&buf)
		if derr != nil {
			cleanup()
			return "", "", func() {}, fmt.Errorf("bmp decode: %w", derr)
		}
		if err := jpeg.Encode(tmp, img, &jpeg.Options{Quality: 92}); err != nil {
			cleanup()
			return "", "", func() {}, fmt.Errorf("jpeg encode: %w", err)
		}
	case len(head) >= 4 && ((head[0] == 0x49 && head[1] == 0x49 && head[2] == 0x2A) || (head[0] == 0x4D && head[1] == 0x4D)):
		img, derr := tiff.Decode(&buf)
		if derr != nil {
			cleanup()
			return "", "", func() {}, fmt.Errorf("tiff decode: %w", derr)
		}
		if err := jpeg.Encode(tmp, img, &jpeg.Options{Quality: 92}); err != nil {
			cleanup()
			return "", "", func() {}, fmt.Errorf("jpeg encode: %w", err)
		}
	default:
		cleanup()
		return "", "", func() {}, fmt.Errorf("unsupported image format (ext=%s, magic=% x)", lower, head[:min(len(head), 8)])
	}

	return tmp.Name(), "JPG", cleanup, nil
}

// Silence "imported and not used" for the decoders we register only for
// their side-effects (image.Decode lookup table).
var (
	_ = png.Encode
	_ = bytes.Buffer{}
)
