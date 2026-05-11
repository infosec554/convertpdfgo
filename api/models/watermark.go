package models

import "time"

// WatermarkRequest is the body for POST /api/pdf/watermark.
// Mode picks between text and image overlay.
type WatermarkRequest struct {
	InputFileID string  `json:"input_file_id" binding:"required"`
	Mode        string  `json:"mode"          binding:"required"` // "text" | "image"
	Text        string  `json:"text"`                             // mode=text
	ImageFileID string  `json:"image_file_id"`                    // mode=image
	Position    string  `json:"position"`                         // c|tl|tr|bl|br (pdfcpu pos string)
	Rotation    int     `json:"rotation"`                         // -90..90
	Opacity     float64 `json:"opacity"`                          // 0..1, default 0.4
	Pages       string  `json:"pages"`                            // "all" or "1-3,5"
}

type WatermarkJob struct {
	ID           string    `json:"id"`
	UserID       *string   `json:"user_id"`
	InputFileID  string    `json:"input_file_id"`
	Mode         string    `json:"mode"`
	Text         string    `json:"text"`
	ImageFileID  *string   `json:"image_file_id"`
	Position     string    `json:"position"`
	Rotation     int       `json:"rotation"`
	Opacity      float64   `json:"opacity"`
	Pages        string    `json:"pages"`
	OutputFileID *string   `json:"output_file_id"`
	Status       JobStatus `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}
