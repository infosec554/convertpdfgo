package models

import "time"

type ExtractJob struct {
	ID             string    `json:"id"`
	UserID         *string   `json:"user_id"`
	InputFileID    string    `json:"input_file_id"`
	PagesToExtract string    `json:"pages_to_extract"`
	OutputFileID   *string   `json:"output_file_id"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
}

type ExtractPagesRequest struct {
	InputFileID string `json:"input_file_id" binding:"required"`
	PageRanges  string `json:"page_ranges" binding:"required"`
}
