package models

import "time"

type HTMLToPDFJob struct {
	ID           string    `json:"id"`
	UserID       *string   `json:"user_id,omitempty"`
	HTMLContent  string    `json:"html_content"`
	OutputFileID *string   `json:"output_file_id,omitempty"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}

type CreateHTMLToPDFRequest struct {
	HTMLContent string  `json:"html_content" binding:"required"`
}
