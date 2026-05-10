package models

import "time"

// UnlockPDFRequest – HTTP orqali yuboriladigan so‘rov
type UnlockPDFRequest struct {
	InputFileID string `json:"input_file_id" binding:"required"` // Kiruvchi PDF fayl IDsi
	Password    string `json:"password" binding:"required"`      // PDF paroli
}

// UnlockPDFJob – Unlock qilish ish modeli
type UnlockPDFJob struct {
	ID           string    `json:"id"`             // Job ID
	UserID       *string    `json:"user_id"`        // Foydalanuvchi IDsi
	InputFileID  string    `json:"input_file_id"`  // Kiruvchi fayl IDsi
	OutputFileID *string    `json:"output_file_id"` // Natijaviy fayl IDsi
	Status       string    `json:"status"`         // pending, done, failed
	CreatedAt    time.Time `json:"created_at"`     // Yaratilgan vaqti
}
