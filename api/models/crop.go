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
	ID           string    `json:"id"`             // Ish IDsi
	UserID       *string   `json:"user_id"`        // Foydalanuvchi IDsi (guest uchun nil bo'lishi mumkin)
	InputFileID  string    `json:"input_file_id"`  // Kirish fayl IDsi
	InputFileIDs []string  `json:"input_file_ids"` // Fayl IDlari ro'yxati (bir nechta fayl uchun)
	OutputFileID *string   `json:"output_file_id"` // Chiqish fayli IDsi
	Top          int       `json:"top"`            // Yuqori chegara
	Bottom       int       `json:"bottom"`         // Pastki chegara
	Left         int       `json:"left"`           // Chap chegara
	Right        int       `json:"right"`          // O'ng chegara
	Box          string    `json:"box"`            // Box turi
	Pages        string    `json:"pages"`          // Sahifalar
	Status       string    `json:"status"`         // Ish holati (masalan, "pending", "done")
	CreatedAt    time.Time `json:"created_at"`     // Yaratilgan vaqt
}
