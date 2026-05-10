package models

import "time"

type AddPageNumbersRequest struct {
	InputFileID string `json:"input_file_id" binding:"required"`
	FirstNumber int    `json:"first_number" binding:"required,min=1"` // <-- bu yer o‘zgardi
	PageRange   string `json:"page_range" binding:"required"`
	Position    string `json:"position" binding:"required"`
	Color       string `json:"color" binding:"required"`
	FontSize    int    `json:"font_size" binding:"required,min=1"` // <-- bu yer o‘zgardi
}

type AddPageNumberJob struct {
	ID           string    `json:"id"`
	UserID       *string   `json:"user_id"`
	InputFileID  string    `json:"input_file_id"`
	OutputFileID *string   `json:"output_file_id"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	FirstNumber  int       `json:"first_number"`
	PageRange    string    `json:"page_range"`
	Position     string    `json:"position"`
	Color        string    `json:"color"`
	FontSize     int       `json:"font_size"`
}
