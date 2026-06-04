package models

import "time"

// PPTXBuilderRequest is the body for POST /api/build/pptx.
// Topic is the only required field; everything else has a sensible default.
type PPTXBuilderRequest struct {
	Topic      string `json:"topic" binding:"required,min=4,max=500"`
	SlideCount int    `json:"slide_count"`                       // 5-15, default 8
	Tone       string `json:"tone"`                              // "corporate" | "modern" | "playful" | "academic"  (default corporate)
	Language   string `json:"language"`                          // "en" | "uz" | "ru" | "es" | "ar" (default en, but LLM auto-detects from topic too)
	Audience   string `json:"audience,omitempty"`                // optional free-text — "executive team", "students", "investors"
}

type PPTXBuilderJob struct {
	ID           string    `json:"id"`
	UserID       *string   `json:"user_id,omitempty"`
	Topic        string    `json:"topic"`
	SlideCount   int       `json:"slide_count"`
	Tone         string    `json:"tone"`
	Language     string    `json:"language"`
	Audience     string    `json:"audience,omitempty"`
	Status       string    `json:"status"` // pending | running | done | failed
	Error        string    `json:"error,omitempty"`
	OutputFileID *string   `json:"output_file_id,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
