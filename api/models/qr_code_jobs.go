package models

import "time"

type CreateQRCodeRequest struct {
	InputFileID string `json:"input_file_id" binding:"required"`
	QRContent   string `json:"qr_content" binding:"required"`
	Position    string `json:"position" binding:"required"` // Masalan: "top-left"
	Size        int    `json:"size" binding:"required"`     // pikselda o'lcham
}

type QRCodeJob struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	InputFileID  string    `json:"input_file_id"`
	QRContent    string    `json:"qr_content"`
	Position     string    `json:"position"`
	Size         int       `json:"size"`
	OutputFileID string    `json:"output_file_id,omitempty"`
	Status       string    `json:"status"` // pending, processing, done, failed
	CreatedAt    time.Time `json:"created_at"`
}
