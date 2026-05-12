// OCR service family. Each of the six is a thin wrapper around
// pkg/ocr.OCR / TextFromImage / DetectLanguage / HOCR plus the
// usual "create job → enqueue → worker processes" plumbing. The
// services live in one file because their Create/Process shapes are
// near-identical; splitting per endpoint would multiply boilerplate
// without buying clarity.
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pdfcpu/pdfcpu/pkg/api"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/pkg/ocr"
	"convertpdfgo/storage"
)

// pickLang defaults to "eng" so handlers can pass an empty string
// when the caller didn't specify a language. Tesseract accepts the
// "+" syntax for multi-language passes, so the value is forwarded
// untouched otherwise.
func pickLang(s string) string {
	if strings.TrimSpace(s) == "" {
		return "eng"
	}
	return s
}

// --- /api/pdf/ocr ----------------------------------------------------------

type OCRService interface {
	Create(ctx context.Context, req models.OCRRequest, userID *string) (string, error)
	Process(ctx context.Context, jobID string) error
	GetByID(ctx context.Context, id string) (*models.OCRJob, error)
}

type ocrService struct {
	stg storage.IStorage
	log logger.ILogger
}

func NewOCRService(stg storage.IStorage, log logger.ILogger) OCRService {
	return &ocrService{stg: stg, log: log}
}

func (s *ocrService) Create(ctx context.Context, req models.OCRRequest, userID *string) (string, error) {
	job := &models.OCRJob{
		ID:          uuid.NewString(),
		UserID:      userID,
		InputFileID: req.InputFileID,
		Lang:        pickLang(req.Lang),
		ForceOCR:    req.ForceOCR,
		Status:      "pending",
		CreatedAt:   time.Now(),
	}
	if err := s.stg.OCR().Create(ctx, job); err != nil {
		return "", err
	}
	return job.ID, nil
}

func (s *ocrService) Process(ctx context.Context, jobID string) error {
	job, err := s.stg.OCR().GetByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("get job: %w", err)
	}
	file, err := s.stg.File().GetByID(ctx, job.InputFileID)
	if err != nil {
		s.markOCRFailed(ctx, job)
		return fmt.Errorf("get input: %w", err)
	}

	outDir := "storage/ocr"
	outID := uuid.NewString()
	outPath := filepath.Join(outDir, outID+".pdf")

	// Default behaviour: skip pages that already have text. The caller
	// opts in to force_ocr if they want to redo a sloppy existing
	// layer.
	if err := ocr.OCR(ctx, file.FilePath, outPath, ocr.Options{
		Languages: job.Lang,
		ForceOCR:  job.ForceOCR,
		SkipText:  !job.ForceOCR,
	}); err != nil {
		s.markOCRFailed(ctx, job)
		return fmt.Errorf("ocrmypdf: %w", err)
	}
	fi, err := os.Stat(outPath)
	if err != nil {
		s.markOCRFailed(ctx, job)
		return fmt.Errorf("stat: %w", err)
	}
	if _, err := s.stg.File().Save(ctx, models.File{
		ID: outID, UserID: job.UserID,
		FileName: filepath.Base(outPath), FilePath: outPath,
		FileType: "application/pdf", FileSize: fi.Size(),
		UploadedAt: time.Now(),
	}); err != nil {
		s.markOCRFailed(ctx, job)
		return fmt.Errorf("save: %w", err)
	}
	job.OutputFileID = &outID
	job.Status = "done"
	return s.stg.OCR().Update(ctx, job)
}

func (s *ocrService) markOCRFailed(ctx context.Context, job *models.OCRJob) {
	job.Status = "failed"
	if err := s.stg.OCR().Update(ctx, job); err != nil {
		s.log.Error("ocr: mark failed", logger.Error(err))
	}
}

func (s *ocrService) GetByID(ctx context.Context, id string) (*models.OCRJob, error) {
	return s.stg.OCR().GetByID(ctx, id)
}

// --- /api/pdf/scan-to-pdf --------------------------------------------------

type ScanToPDFService interface {
	Create(ctx context.Context, req models.ScanToPDFRequest, userID *string) (string, error)
	Process(ctx context.Context, jobID string) error
	GetByID(ctx context.Context, id string) (*models.ScanToPDFJob, error)
}

type scanToPDFService struct {
	stg storage.IStorage
	log logger.ILogger
}

func NewScanToPDFService(stg storage.IStorage, log logger.ILogger) ScanToPDFService {
	return &scanToPDFService{stg: stg, log: log}
}

func (s *scanToPDFService) Create(ctx context.Context, req models.ScanToPDFRequest, userID *string) (string, error) {
	job := &models.ScanToPDFJob{
		ID:           uuid.NewString(),
		UserID:       userID,
		InputFileIDs: req.InputFileIDs,
		Lang:         pickLang(req.Lang),
		Status:       "pending",
		CreatedAt:    time.Now(),
	}
	if err := s.stg.ScanToPDF().Create(ctx, job); err != nil {
		return "", err
	}
	return job.ID, nil
}

func (s *scanToPDFService) Process(ctx context.Context, jobID string) error {
	job, err := s.stg.ScanToPDF().GetByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("get job: %w", err)
	}

	// Gather image paths in the order the client gave us.
	paths := make([]string, 0, len(job.InputFileIDs))
	for _, id := range job.InputFileIDs {
		f, err := s.stg.File().GetByID(ctx, id)
		if err != nil {
			s.markScanFailed(ctx, job)
			return fmt.Errorf("get input %s: %w", id, err)
		}
		paths = append(paths, f.FilePath)
	}

	outDir := "storage/scan_to_pdf"
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		s.markScanFailed(ctx, job)
		return fmt.Errorf("mkdir: %w", err)
	}
	intermediate := filepath.Join(outDir, "raw-"+uuid.NewString()+".pdf")
	// pdfcpu Import puts each image on its own page. Same routine
	// jpg-to-pdf uses, so the result is a well-formed PDF.
	if err := api.ImportImagesFile(paths, intermediate, nil, nil); err != nil {
		s.markScanFailed(ctx, job)
		return fmt.Errorf("images→pdf: %w", err)
	}
	defer os.Remove(intermediate)

	outID := uuid.NewString()
	outPath := filepath.Join(outDir, outID+".pdf")
	// The image-built PDF has no text layer; --force-ocr would also
	// work, but the default skip-text path is fine because every page
	// is a pure image.
	if err := ocr.OCR(ctx, intermediate, outPath, ocr.Options{
		Languages: job.Lang,
		ForceOCR:  true, // images have no existing text — explicit is clearer
	}); err != nil {
		s.markScanFailed(ctx, job)
		return fmt.Errorf("ocrmypdf: %w", err)
	}
	fi, err := os.Stat(outPath)
	if err != nil {
		s.markScanFailed(ctx, job)
		return fmt.Errorf("stat: %w", err)
	}
	if _, err := s.stg.File().Save(ctx, models.File{
		ID: outID, UserID: job.UserID,
		FileName: filepath.Base(outPath), FilePath: outPath,
		FileType: "application/pdf", FileSize: fi.Size(),
		UploadedAt: time.Now(),
	}); err != nil {
		s.markScanFailed(ctx, job)
		return fmt.Errorf("save: %w", err)
	}
	job.OutputFileID = &outID
	job.Status = "done"
	return s.stg.ScanToPDF().Update(ctx, job)
}

func (s *scanToPDFService) markScanFailed(ctx context.Context, job *models.ScanToPDFJob) {
	job.Status = "failed"
	if err := s.stg.ScanToPDF().Update(ctx, job); err != nil {
		s.log.Error("scan_to_pdf: mark failed", logger.Error(err))
	}
}

func (s *scanToPDFService) GetByID(ctx context.Context, id string) (*models.ScanToPDFJob, error) {
	return s.stg.ScanToPDF().GetByID(ctx, id)
}

// --- /api/pdf/ocr-extract-text --------------------------------------------

type OCRExtractTextService interface {
	Create(ctx context.Context, req models.OCRExtractTextRequest, userID *string) (string, error)
	Process(ctx context.Context, jobID string) error
	GetByID(ctx context.Context, id string) (*models.OCRExtractTextJob, error)
}

type ocrExtractTextService struct {
	stg storage.IStorage
	log logger.ILogger
}

func NewOCRExtractTextService(stg storage.IStorage, log logger.ILogger) OCRExtractTextService {
	return &ocrExtractTextService{stg: stg, log: log}
}

func (s *ocrExtractTextService) Create(ctx context.Context, req models.OCRExtractTextRequest, userID *string) (string, error) {
	job := &models.OCRExtractTextJob{
		ID:          uuid.NewString(),
		UserID:      userID,
		InputFileID: req.InputFileID,
		Lang:        pickLang(req.Lang),
		Status:      "pending",
		CreatedAt:   time.Now(),
	}
	if err := s.stg.OCRExtractText().Create(ctx, job); err != nil {
		return "", err
	}
	return job.ID, nil
}

func (s *ocrExtractTextService) Process(ctx context.Context, jobID string) error {
	job, err := s.stg.OCRExtractText().GetByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("get job: %w", err)
	}
	file, err := s.stg.File().GetByID(ctx, job.InputFileID)
	if err != nil {
		s.markETFailed(ctx, job)
		return fmt.Errorf("get input: %w", err)
	}

	outDir := "storage/ocr_extract_text"
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		s.markETFailed(ctx, job)
		return fmt.Errorf("mkdir: %w", err)
	}
	outID := uuid.NewString()
	sidecarPath := filepath.Join(outDir, outID+".txt")
	// ocrmypdf can drop the OCR'd text into a sidecar in one shot, no
	// need to run tesseract twice. The PDF output is throwaway here.
	tmpPDF := filepath.Join(outDir, "tmp-"+outID+".pdf")
	defer os.Remove(tmpPDF)

	if err := ocr.OCR(ctx, file.FilePath, tmpPDF, ocr.Options{
		Languages: job.Lang,
		SkipText:  false,
		ForceOCR:  true,
		Sidecar:   sidecarPath,
	}); err != nil {
		s.markETFailed(ctx, job)
		return fmt.Errorf("ocrmypdf: %w", err)
	}
	fi, err := os.Stat(sidecarPath)
	if err != nil {
		s.markETFailed(ctx, job)
		return fmt.Errorf("stat sidecar: %w", err)
	}
	if _, err := s.stg.File().Save(ctx, models.File{
		ID: outID, UserID: job.UserID,
		FileName: filepath.Base(sidecarPath), FilePath: sidecarPath,
		FileType: "text/plain", FileSize: fi.Size(),
		UploadedAt: time.Now(),
	}); err != nil {
		s.markETFailed(ctx, job)
		return fmt.Errorf("save: %w", err)
	}
	job.OutputFileID = &outID
	job.Status = "done"
	return s.stg.OCRExtractText().Update(ctx, job)
}

func (s *ocrExtractTextService) markETFailed(ctx context.Context, job *models.OCRExtractTextJob) {
	job.Status = "failed"
	if err := s.stg.OCRExtractText().Update(ctx, job); err != nil {
		s.log.Error("ocr_extract_text: mark failed", logger.Error(err))
	}
}

func (s *ocrExtractTextService) GetByID(ctx context.Context, id string) (*models.OCRExtractTextJob, error) {
	return s.stg.OCRExtractText().GetByID(ctx, id)
}

// --- /api/pdf/image-to-text -----------------------------------------------

type ImageToTextService interface {
	Create(ctx context.Context, req models.ImageToTextRequest, userID *string) (string, error)
	Process(ctx context.Context, jobID string) error
	GetByID(ctx context.Context, id string) (*models.ImageToTextJob, error)
}

type imageToTextService struct {
	stg storage.IStorage
	log logger.ILogger
}

func NewImageToTextService(stg storage.IStorage, log logger.ILogger) ImageToTextService {
	return &imageToTextService{stg: stg, log: log}
}

func (s *imageToTextService) Create(ctx context.Context, req models.ImageToTextRequest, userID *string) (string, error) {
	job := &models.ImageToTextJob{
		ID:          uuid.NewString(),
		UserID:      userID,
		InputFileID: req.InputFileID,
		Lang:        pickLang(req.Lang),
		Status:      "pending",
		CreatedAt:   time.Now(),
	}
	if err := s.stg.ImageToText().Create(ctx, job); err != nil {
		return "", err
	}
	return job.ID, nil
}

func (s *imageToTextService) Process(ctx context.Context, jobID string) error {
	job, err := s.stg.ImageToText().GetByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("get job: %w", err)
	}
	file, err := s.stg.File().GetByID(ctx, job.InputFileID)
	if err != nil {
		s.markI2TFailed(ctx, job)
		return fmt.Errorf("get input: %w", err)
	}

	text, err := ocr.TextFromImage(ctx, file.FilePath, job.Lang)
	if err != nil {
		s.markI2TFailed(ctx, job)
		return fmt.Errorf("tesseract: %w", err)
	}

	outDir := "storage/image_to_text"
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		s.markI2TFailed(ctx, job)
		return fmt.Errorf("mkdir: %w", err)
	}
	outID := uuid.NewString()
	outPath := filepath.Join(outDir, outID+".txt")
	if err := os.WriteFile(outPath, []byte(text), 0o644); err != nil {
		s.markI2TFailed(ctx, job)
		return fmt.Errorf("write text: %w", err)
	}
	if _, err := s.stg.File().Save(ctx, models.File{
		ID: outID, UserID: job.UserID,
		FileName: filepath.Base(outPath), FilePath: outPath,
		FileType: "text/plain", FileSize: int64(len(text)),
		UploadedAt: time.Now(),
	}); err != nil {
		s.markI2TFailed(ctx, job)
		return fmt.Errorf("save: %w", err)
	}
	job.OutputFileID = &outID
	job.Status = "done"
	return s.stg.ImageToText().Update(ctx, job)
}

func (s *imageToTextService) markI2TFailed(ctx context.Context, job *models.ImageToTextJob) {
	job.Status = "failed"
	if err := s.stg.ImageToText().Update(ctx, job); err != nil {
		s.log.Error("image_to_text: mark failed", logger.Error(err))
	}
}

func (s *imageToTextService) GetByID(ctx context.Context, id string) (*models.ImageToTextJob, error) {
	return s.stg.ImageToText().GetByID(ctx, id)
}

// --- /api/pdf/ocr-detect-lang ---------------------------------------------

type OCRDetectLangService interface {
	Create(ctx context.Context, req models.OCRDetectLangRequest, userID *string) (string, error)
	Process(ctx context.Context, jobID string) error
	GetByID(ctx context.Context, id string) (*models.OCRDetectLangJob, error)
}

type ocrDetectLangService struct {
	stg storage.IStorage
	log logger.ILogger
}

func NewOCRDetectLangService(stg storage.IStorage, log logger.ILogger) OCRDetectLangService {
	return &ocrDetectLangService{stg: stg, log: log}
}

func (s *ocrDetectLangService) Create(ctx context.Context, req models.OCRDetectLangRequest, userID *string) (string, error) {
	job := &models.OCRDetectLangJob{
		ID:          uuid.NewString(),
		UserID:      userID,
		InputFileID: req.InputFileID,
		Status:      "pending",
		CreatedAt:   time.Now(),
	}
	if err := s.stg.OCRDetectLang().Create(ctx, job); err != nil {
		return "", err
	}
	return job.ID, nil
}

func (s *ocrDetectLangService) Process(ctx context.Context, jobID string) error {
	job, err := s.stg.OCRDetectLang().GetByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("get job: %w", err)
	}
	file, err := s.stg.File().GetByID(ctx, job.InputFileID)
	if err != nil {
		s.markDLFailed(ctx, job)
		return fmt.Errorf("get input: %w", err)
	}

	// Tesseract OSD needs an image. If the input is a PDF, rasterise
	// the first page to PNG via poppler first — same dependency we
	// already lean on for pdf-to-jpg / extract-text.
	imgPath := file.FilePath
	if strings.EqualFold(filepath.Ext(file.FilePath), ".pdf") {
		tmpDir, err := os.MkdirTemp("", "osd-")
		if err != nil {
			s.markDLFailed(ctx, job)
			return fmt.Errorf("mktmp: %w", err)
		}
		defer os.RemoveAll(tmpDir)
		// pdftoppm writes <stem>-1.png for the first page; -singlefile
		// drops the suffix and writes <stem>.png exactly.
		imgPath = filepath.Join(tmpDir, "page.png")
		if err := pdftoppmFirstPage(ctx, file.FilePath, strings.TrimSuffix(imgPath, ".png")); err != nil {
			s.markDLFailed(ctx, job)
			return fmt.Errorf("rasterise: %w", err)
		}
	}

	lang, conf, err := ocr.DetectLanguage(ctx, imgPath)
	if err != nil {
		s.markDLFailed(ctx, job)
		return fmt.Errorf("osd: %w", err)
	}
	job.DetectedLang = &lang
	job.Confidence = &conf
	job.Status = "done"
	return s.stg.OCRDetectLang().Update(ctx, job)
}

func (s *ocrDetectLangService) markDLFailed(ctx context.Context, job *models.OCRDetectLangJob) {
	job.Status = "failed"
	if err := s.stg.OCRDetectLang().Update(ctx, job); err != nil {
		s.log.Error("ocr_detect_lang: mark failed", logger.Error(err))
	}
}

func (s *ocrDetectLangService) GetByID(ctx context.Context, id string) (*models.OCRDetectLangJob, error) {
	return s.stg.OCRDetectLang().GetByID(ctx, id)
}

// --- /api/pdf/ocr-structured ----------------------------------------------

type OCRStructuredService interface {
	Create(ctx context.Context, req models.OCRStructuredRequest, userID *string) (string, error)
	Process(ctx context.Context, jobID string) error
	GetByID(ctx context.Context, id string) (*models.OCRStructuredJob, error)
}

type ocrStructuredService struct {
	stg storage.IStorage
	log logger.ILogger
}

func NewOCRStructuredService(stg storage.IStorage, log logger.ILogger) OCRStructuredService {
	return &ocrStructuredService{stg: stg, log: log}
}

func (s *ocrStructuredService) Create(ctx context.Context, req models.OCRStructuredRequest, userID *string) (string, error) {
	job := &models.OCRStructuredJob{
		ID:          uuid.NewString(),
		UserID:      userID,
		InputFileID: req.InputFileID,
		Lang:        pickLang(req.Lang),
		Status:      "pending",
		CreatedAt:   time.Now(),
	}
	if err := s.stg.OCRStructured().Create(ctx, job); err != nil {
		return "", err
	}
	return job.ID, nil
}

// hOCRPayload is the minimal JSON wrapper we serve at the /file
// endpoint — clients can re-parse the embedded hocr string with any
// HTML parser if they want bounding boxes per word.
type hOCRPayload struct {
	Lang  string `json:"lang"`
	HOCR  string `json:"hocr"`
	Note  string `json:"note"`
}

func (s *ocrStructuredService) Process(ctx context.Context, jobID string) error {
	job, err := s.stg.OCRStructured().GetByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("get job: %w", err)
	}
	file, err := s.stg.File().GetByID(ctx, job.InputFileID)
	if err != nil {
		s.markStructFailed(ctx, job)
		return fmt.Errorf("get input: %w", err)
	}

	// Rasterise PDFs (tesseract HOCR mode needs an image). For images
	// we use the file directly.
	imgPath := file.FilePath
	if strings.EqualFold(filepath.Ext(file.FilePath), ".pdf") {
		tmpDir, err := os.MkdirTemp("", "hocr-")
		if err != nil {
			s.markStructFailed(ctx, job)
			return fmt.Errorf("mktmp: %w", err)
		}
		defer os.RemoveAll(tmpDir)
		imgPath = filepath.Join(tmpDir, "page.png")
		if err := pdftoppmFirstPage(ctx, file.FilePath, strings.TrimSuffix(imgPath, ".png")); err != nil {
			s.markStructFailed(ctx, job)
			return fmt.Errorf("rasterise: %w", err)
		}
	}

	outDir := "storage/ocr_structured"
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		s.markStructFailed(ctx, job)
		return fmt.Errorf("mkdir: %w", err)
	}
	outID := uuid.NewString()
	hocrPath := filepath.Join(outDir, outID+".hocr")
	if err := ocr.HOCR(ctx, imgPath, hocrPath, job.Lang); err != nil {
		s.markStructFailed(ctx, job)
		return fmt.Errorf("hocr: %w", err)
	}
	hocrBytes, err := os.ReadFile(hocrPath)
	if err != nil {
		s.markStructFailed(ctx, job)
		return fmt.Errorf("read hocr: %w", err)
	}
	// Wrap in JSON so clients consuming "structured output" don't need
	// to special-case .hocr's HTML envelope. /file/{id}/download
	// returns this directly.
	jsonPath := filepath.Join(outDir, outID+".json")
	payload, _ := json.MarshalIndent(hOCRPayload{
		Lang: job.Lang,
		HOCR: string(hocrBytes),
		Note: "hocr is tesseract HTML output; parse for word-level bounding boxes",
	}, "", "  ")
	if err := os.WriteFile(jsonPath, payload, 0o644); err != nil {
		s.markStructFailed(ctx, job)
		return fmt.Errorf("write json: %w", err)
	}
	_ = os.Remove(hocrPath) // the json wraps everything we need

	if _, err := s.stg.File().Save(ctx, models.File{
		ID: outID, UserID: job.UserID,
		FileName: filepath.Base(jsonPath), FilePath: jsonPath,
		FileType: "application/json", FileSize: int64(len(payload)),
		UploadedAt: time.Now(),
	}); err != nil {
		s.markStructFailed(ctx, job)
		return fmt.Errorf("save: %w", err)
	}
	job.OutputFileID = &outID
	job.Status = "done"
	return s.stg.OCRStructured().Update(ctx, job)
}

func (s *ocrStructuredService) markStructFailed(ctx context.Context, job *models.OCRStructuredJob) {
	job.Status = "failed"
	if err := s.stg.OCRStructured().Update(ctx, job); err != nil {
		s.log.Error("ocr_structured: mark failed", logger.Error(err))
	}
}

func (s *ocrStructuredService) GetByID(ctx context.Context, id string) (*models.OCRStructuredJob, error) {
	return s.stg.OCRStructured().GetByID(ctx, id)
}

// pdftoppmFirstPage renders only page 1 of a PDF as a PNG. The output
// path is passed without an extension; pdftoppm appends .png itself.
// Shared by ocr-detect-lang and ocr-structured.
func pdftoppmFirstPage(ctx context.Context, srcPDF, outStem string) error {
	rctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// `-r 200` keeps the OCR target high-DPI enough that script
	// detection / per-word bboxes are stable. `-f 1 -l 1` clamps to
	// the first page; `-singlefile` drops the -N page-number suffix.
	args := []string{"-png", "-r", "200", "-f", "1", "-l", "1", "-singlefile", srcPDF, outStem}
	cmd := exec.CommandContext(rctx, "pdftoppm", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("pdftoppm: %w; out: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
