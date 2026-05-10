package models

import "time"

// Define compression levels as a custom type
type CompressionLevel string

const (
	Low    CompressionLevel = "low"
	Medium CompressionLevel = "medium"
	High   CompressionLevel = "high"
)

// Define job status as a custom type
type JobStatus string

const (
	Pending    JobStatus = "pending"
	Processing JobStatus = "processing"
	Done       JobStatus = "done"
	Failed     JobStatus = "failed"
)

// CompressJob struct represents a compression job
type CompressJob struct {
	ID               string           `json:"id"`
	UserID           *string          `json:"user_id"` // Can be nil for guest users
	InputFileID      string           `json:"input_file_id"`
	OutputFileID     *string          `json:"output_file_id"` // Output file, can be nil
	CompressionLevel CompressionLevel `json:"compression"`    // "low", "medium", "high"
	Status           JobStatus        `json:"status"`         // "pending", "processing", "done", "failed"
	CreatedAt        time.Time        `json:"created_at"`
}

// CompressRequest struct represents the request to start a compression job
type CompressRequest struct {
	InputFileID string           `json:"input_file_id" binding:"required"`
	Compression CompressionLevel `json:"compression" binding:"required"` // "low", "medium", "high"
}
