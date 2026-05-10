package models

import "time"

// ProtectPDFRequest – Kiruvchi so‘rov modeli
type ProtectPDFRequest struct {
	InputFileID string `json:"input_file_id" binding:"required"` // PDF fayl ID
	Password    string `json:"password" binding:"required"`      // Qo‘yiladigan parol
}

type ProtectPDFJob struct {
	ID           string    `json:"id"`
	UserID       *string   `json:"user_id"`
	InputFileID  string    `json:"input_file_id"`
	OutputFileID *string   `json:"output_file_id"` // ← *pointer (✔ to‘g‘ri)
	Password     string    `json:"password"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}
