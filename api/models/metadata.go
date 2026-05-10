package models

import "time"

type MetadataRequest struct {
	InputFileID string   `json:"input_file_id" binding:"required"`
	Title       *string  `json:"title"`
	Author      *string  `json:"author"`
	Subject     *string  `json:"subject"`
	Keywords    []string `json:"keywords"`
	Creator     *string  `json:"creator"`
}

type MetadataJob struct {
	ID           string    `json:"id"`
	UserID       *string   `json:"user_id"`
	InputFileID  string    `json:"input_file_id"`
	Title        *string   `json:"title"`
	Author       *string   `json:"author"`
	Subject      *string   `json:"subject"`
	Keywords     *string   `json:"keywords"`
	Creator      *string   `json:"creator"`
	OutputFileID *string   `json:"output_file_id"`
	Status       JobStatus `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}
