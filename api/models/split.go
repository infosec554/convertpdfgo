package models

import "time"

type CreateSplitJobRequest struct {
	InputFileID string `json:"input_file_id" binding:"required,uuid"`
	SplitRanges string `json:"split_ranges" binding:"required"` // Masalan: "1-3,4-5"
}

type SplitJob struct {
	ID            string    `json:"id"`
	UserID        *string   `json:"user_id"` // ❗️ string emas, *string bo‘lishi kerak
	InputFileID   string    `json:"input_file_id"`
	SplitRanges   string    `json:"split_ranges"`
	OutputFileIDs []string  `json:"output_file_ids"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}
