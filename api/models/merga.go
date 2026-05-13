package models

import "time"

// MergeJob – bitta birlashtirish (merge) vazifasining asosiy ma'lumotlari
type MergeJob struct {
	ID           string    `json:"id"            db:"id"`
	UserID       *string   `json:"user_id"       db:"user_id"`         // NULL bo'lishi mumkin (guest)
	OutputFileID *string   `json:"output_file_id" db:"output_file_id"` // NULL bo'lishi mumkin (hozircha natija yo'q)
	Status       string    `json:"status"        db:"status"`          // pending, done, failed
	CreatedAt    time.Time `json:"created_at"    db:"created_at"`
	InputFileIDs []string  `json:"input_file_ids" db:"-"` // JOIN orqali olinadi, db da yo'q
}

// CreateMergeJobRequest – API orqali kiritiladigan inputlar.
//
// InputFileIDs: 2..50 ta fayl. Yuqori chegara — abuse / DoS himoyasi.
// 50 ta PDF birlashishi normal foydalanish chegarasidan oshib ketmaydi
// va worker pdfcpu uchun makul yuk.
type CreateMergeJobRequest struct {
	UserID       *string  `json:"user_id"`
	InputFileIDs []string `json:"input_file_ids" binding:"required,min=2,max=50,dive,uuid"`
}
