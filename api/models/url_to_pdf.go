package models

import "time"

type URLToPDFRequest struct {
	URL       string `json:"url"        binding:"required"`
	PageSize  string `json:"page_size"` // default "A4"
	Landscape bool   `json:"landscape"`
	WaitFor   string `json:"wait_for"` // optional Gotenberg waitDelay or selector
}

type URLToPDFJob struct {
	ID           string    `json:"id"`
	UserID       *string   `json:"user_id"`
	URL          string    `json:"url"`
	PageSize     string    `json:"page_size"`
	Landscape    bool      `json:"landscape"`
	WaitFor      *string   `json:"wait_for"`
	OutputFileID *string   `json:"output_file_id"`
	Status       JobStatus `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}
