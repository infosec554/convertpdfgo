package models

import "time"

type CreateAddBackgroundRequest struct {
	InputFileID           string  `json:"input_file_id" binding:"required"`
	BackgroundImageFileID string  `json:"background_image_file_id" binding:"required"`
	Opacity               float64 `json:"opacity,omitempty"`  // 0.0 - 1.0
	Position              string  `json:"position,omitempty"` // center, top-left, bottom-right
}

type AddBackgroundJob struct {
	ID                    string    `json:"id"`
	UserID                *string    `json:"user_id"`
	InputFileID           string    `json:"input_file_id"`
	BackgroundImageFileID string    `json:"background_image_file_id"`
	Opacity               float64   `json:"opacity"`
	Position              string    `json:"position"`
	OutputFileID          *string    `json:"output_file_id,omitempty"`
	Status                string    `json:"status"`
	CreatedAt             time.Time `json:"created_at"`
}
