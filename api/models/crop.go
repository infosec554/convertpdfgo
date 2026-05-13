package models

import "time"

// CropPDFRequest – HTTP orqali keladigan crop so‘rovi
type CropPDFRequest struct {
	InputFileID string `json:"input_file_id" binding:"required"`
	Top         int    `json:"top"`
	Bottom      int    `json:"bottom"`
	Left        int    `json:"left"`
	Right       int    `json:"right"`

	Pages string `json:"pages"` // optional: qaysi sahifalarni crop qilish
	Box   string `json:"box"`   // optional: mediabox, cropbox, etc
}

// CropPDFJob – Crop PDF uchun bajariladigan ish modeli

type CropPDFJob struct {
	ID           string    `json:"id"`
	UserID       *string   `json:"user_id"`
	InputFileID  string    `json:"input_file_id"`
	OutputFileID *string   `json:"output_file_id"`
	Top          int       `json:"top"`
	Bottom       int       `json:"bottom"`
	Left         int       `json:"left"`
	Right        int       `json:"right"`
	Box          string    `json:"box"`
	Pages        string    `json:"pages"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}
