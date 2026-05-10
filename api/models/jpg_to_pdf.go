package models

import "time"

type JPGToPDFJob struct {
	ID           string    `json:"id"`
	UserID       *string   `json:"user_id"`
	InputFileIDs []string  `json:"input_file_ids"`
	OutputFileID *string   `json:"output_file_id,omitempty"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}
type CreateJPGToPDFRequest struct {
	InputFileIDs []string `json:"input_file_ids" binding:"required"` // JPG fayllar bir nechta bo‘ladi
}
