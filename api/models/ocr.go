package models

import "time"

// --- /api/pdf/ocr ----------------------------------------------------------

type OCRRequest struct {
	InputFileID string `json:"input_file_id" binding:"required"`
	// Tesseract languages, "+" separated. Empty defaults to "eng".
	// Examples: "eng", "eng+uzb", "rus+uzb_cyrl".
	Lang     string `json:"lang"`
	ForceOCR bool   `json:"force_ocr"` // re-OCR pages that already have a text layer
}

type OCRJob struct {
	ID           string    `json:"id"`
	UserID       *string   `json:"user_id,omitempty"`
	InputFileID  string    `json:"input_file_id"`
	OutputFileID *string   `json:"output_file_id,omitempty"`
	Lang         string    `json:"lang"`
	ForceOCR     bool      `json:"force_ocr"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}

// --- /api/pdf/scan-to-pdf --------------------------------------------------

type ScanToPDFRequest struct {
	// Bir nechta yuklangan rasm IDsi (JPG/PNG) — har biri bir sahifa.
	// 1..50 ta — image batch'i bilan bir xil chegarada.
	InputFileIDs []string `json:"input_file_ids" binding:"required,min=1,max=50,dive,uuid"`
	Lang         string   `json:"lang"`
}

type ScanToPDFJob struct {
	ID            string    `json:"id"`
	UserID        *string   `json:"user_id,omitempty"`
	InputFileIDs  []string  `json:"input_file_ids"`
	OutputFileID  *string   `json:"output_file_id,omitempty"`
	Lang          string    `json:"lang"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}

// --- /api/pdf/ocr-extract-text --------------------------------------------

type OCRExtractTextRequest struct {
	InputFileID string `json:"input_file_id" binding:"required"`
	Lang        string `json:"lang"`
}

type OCRExtractTextJob struct {
	ID           string    `json:"id"`
	UserID       *string   `json:"user_id,omitempty"`
	InputFileID  string    `json:"input_file_id"`
	OutputFileID *string   `json:"output_file_id,omitempty"`
	Lang         string    `json:"lang"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}

// --- /api/pdf/image-to-text -----------------------------------------------

type ImageToTextRequest struct {
	InputFileID string `json:"input_file_id" binding:"required"`
	Lang        string `json:"lang"`
}

type ImageToTextJob struct {
	ID           string    `json:"id"`
	UserID       *string   `json:"user_id,omitempty"`
	InputFileID  string    `json:"input_file_id"`
	OutputFileID *string   `json:"output_file_id,omitempty"`
	Lang         string    `json:"lang"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}

// --- /api/pdf/ocr-detect-lang ---------------------------------------------

type OCRDetectLangRequest struct {
	InputFileID string `json:"input_file_id" binding:"required"`
}

type OCRDetectLangJob struct {
	ID           string    `json:"id"`
	UserID       *string   `json:"user_id,omitempty"`
	InputFileID  string    `json:"input_file_id"`
	DetectedLang *string   `json:"detected_lang,omitempty"`
	Confidence   *float64  `json:"confidence,omitempty"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}

// --- /api/pdf/ocr-structured ----------------------------------------------

type OCRStructuredRequest struct {
	InputFileID string `json:"input_file_id" binding:"required"`
	Lang        string `json:"lang"`
}

type OCRStructuredJob struct {
	ID           string    `json:"id"`
	UserID       *string   `json:"user_id,omitempty"`
	InputFileID  string    `json:"input_file_id"`
	OutputFileID *string   `json:"output_file_id,omitempty"`
	Lang         string    `json:"lang"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}
