package models

import "time"

type PDFTextSearchJob struct {
	ID            string    `json:"id"`
	UserID        *string    `json:"user_id"`
	InputFileID   string    `json:"input_file_id"`
	ExtractedText string    `json:"extracted_text,omitempty"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}
type CreatePDFTextSearchRequest struct {
	InputFileID string `json:"input_file_id" binding:"required"` //
}
