package models

import "time"

// XLSXBuilderRequest is the body for POST /api/build/xlsx.
type XLSXBuilderRequest struct {
	Topic        string `json:"topic" binding:"required,min=4,max=500"`
	SheetCount   int    `json:"sheet_count"`            // 1-5, default 2
	Tone         string `json:"tone"`                   // "corporate" | "modern" | "playful" | "academic"
	Language     string `json:"language"`               // en | uz | ru | es | ar
	Audience     string `json:"audience,omitempty"`
	IncludeChart bool   `json:"include_chart,omitempty"` // default true
}

type XLSXBuilderJob struct {
	ID           string    `json:"id"`
	UserID       *string   `json:"user_id,omitempty"`
	Topic        string    `json:"topic"`
	SheetCount   int       `json:"sheet_count"`
	Tone         string    `json:"tone"`
	Language     string    `json:"language"`
	Audience     string    `json:"audience,omitempty"`
	IncludeChart bool      `json:"include_chart"`
	Status       string    `json:"status"`
	Error        string    `json:"error,omitempty"`
	OutputFileID *string   `json:"output_file_id,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
