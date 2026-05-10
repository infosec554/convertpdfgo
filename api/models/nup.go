package models

import "time"

type NUpRequest struct {
	InputFileID string `json:"input_file_id" binding:"required"`
	N           int    `json:"n"           binding:"required"` // 2,4,6,9
	Orientation string `json:"orientation"`                    // "portrait"|"landscape"
	Border      bool   `json:"border"`
}

type NUpJob struct {
	ID           string    `json:"id"`
	UserID       *string   `json:"user_id"`
	InputFileID  string    `json:"input_file_id"`
	N            int       `json:"n"`
	Orientation  string    `json:"orientation"`
	Border       bool      `json:"border"`
	OutputFileID *string   `json:"output_file_id"`
	Status       JobStatus `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}
