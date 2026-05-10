package models

import "time"

// PDFToJPGJob – PDF dan JPGga aylantirish uchun job modeli
type PDFToJPGJob struct {
	ID            string    `json:"id"`              // Job ID
	UserID        *string   `json:"user_id"`         // User ID (nullable)
	InputFileID   string    `json:"input_file_id"`   // Input file ID
	OutputFileIDs []string  `json:"output_file_ids"` // List of generated JPG file IDs
	ZipFileID     *string   `json:"zip_file_id"`     // ZIP file ID (nullable, can be nil)
	Status        string    `json:"status"`          // Status of the job (pending, processing, done, failed)
	CreatedAt     time.Time `json:"created_at"`      // Job creation timestamp
}

// PDFToJPGRequest – mijozdan keladigan request
type PDFToJPGRequest struct {
	InputFileID string `json:"input_file_id" binding:"required"` // Required input file ID
}
