package models

import "time"

// SignRequest is the input shape for /api/pdf/sign — place a signature
// image (already uploaded) onto a specific page of an input PDF at
// absolute coordinates measured in PDF points from the page's
// bottom-left origin.
type SignRequest struct {
	InputFileID     string  `json:"input_file_id" binding:"required"`
	SignatureFileID string  `json:"signature_file_id" binding:"required"`
	Page            int     `json:"page" binding:"required,min=1"`
	X               float64 `json:"x" binding:"min=0"`
	Y               float64 `json:"y" binding:"min=0"`
	Width           float64 `json:"width" binding:"required,min=10,max=1000"`
}

// SignJob is the persisted record for a signing operation. Width is
// the absolute signature width in PDF points; height is derived from
// the signature image's aspect ratio at render time.
type SignJob struct {
	ID              string     `db:"id"`
	UserID          *string    `db:"user_id"`
	InputFileID     string     `db:"input_file_id"`
	SignatureFileID string     `db:"signature_file_id"`
	Page            int        `db:"page"`
	X               float64    `db:"x"`
	Y               float64    `db:"y"`
	Width           float64    `db:"width"`
	OutputFileID    *string    `db:"output_file_id"`
	Status          JobStatus  `db:"status"`
	CreatedAt       time.Time  `db:"created_at"`
}
