package models

import "time"

type TranslatePDFRequest struct {
	InputFileID string `json:"input_file_id" binding:"required"`
	SourceLang  string `json:"source_lang" binding:"required"`
	TargetLang  string `json:"target_lang" binding:"required"`
}

type TranslatePDFJob struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	InputFileID  string    `json:"input_file_id"`
	SourceLang   string    `json:"source_lang"`
	TargetLang   string    `json:"target_lang"`
	OutputFileID string    `json:"output_file_id"`
	Status       string    `json:"status"` // pending, processing, done, failed
	CreatedAt    time.Time `json:"created_at"`
}
