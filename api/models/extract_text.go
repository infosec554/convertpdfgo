package models

import "time"

type ExtractTextRequest struct {
	InputFileID string `json:"input_file_id" binding:"required"`
	Format      string `json:"format"` // "txt" (default) | "json"
}

type ExtractTextJob struct {
	ID           string    `json:"id"`
	UserID       *string   `json:"user_id"`
	InputFileID  string    `json:"input_file_id"`
	Format       string    `json:"format"`
	OutputFileID *string   `json:"output_file_id"`
	Status       JobStatus `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}
