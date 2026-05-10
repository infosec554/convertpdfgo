package models

import "time"

type PowerPointToPDFRequest struct {
	InputFileID string `json:"input_file_id" binding:"required,uuid"`
}

type PowerPointToPDFJob struct {
	ID           string    `json:"id"`
	UserID       *string   `json:"user_id,omitempty"`
	InputFileID  string    `json:"input_file_id"`
	OutputFileID *string   `json:"output_file_id,omitempty"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}
