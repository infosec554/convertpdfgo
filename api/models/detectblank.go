package models

import "time"

// DetectBlankPagesRequest – foydalanuvchidan keladigan so‘rov
type DetectBlankPagesRequest struct {
	InputFileID string `json:"input_file_id" binding:"required"`
}

// DetectBlankPagesJob – bo‘sh sahifalarni aniqlash jarayoni modeli
type DetectBlankPagesJob struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	InputFileID string    `json:"input_file_id"`
	BlankPages  []int     `json:"blank_pages"`
	Status      string    `json:"status"` // pending, processing, done, failed
	CreatedAt   time.Time `json:"created_at"`
}
