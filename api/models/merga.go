package models

import "time"

// MergeJob – bitta birlashtirish (merge) vazifasining asosiy ma'lumotlari
type MergeJob struct {
	ID           string    `db:"id"`
	UserID       *string   `db:"user_id"`        // NULL bo'lishi mumkin (guest)
	OutputFileID *string   `db:"output_file_id"` // NULL bo'lishi mumkin (hozircha natija yo'q)
	Status       string    `db:"status"`         // pending, done, failed
	CreatedAt    time.Time `db:"created_at"`
	InputFileIDs []string  `db:"-"` // input fayllar (JOIN orqali olinadi, db da yo‘q)
}

// MergeJobInputFile – `merge_job_input_files` jadvalidagi har bir satr
type MergeJobInputFile struct {
	ID     string `db:"id"`
	JobID  string `db:"job_id"`
	FileID string `db:"file_id"`
}

// CreateMergeJobRequest – API orqali kiritiladigan inputlar
type CreateMergeJobRequest struct {
	UserID       *string  `json:"user_id"`        // optional (guest user uchun nil bo'lishi mumkin)
	InputFileIDs []string `json:"input_file_ids"` // birlashtirish uchun kerakli fayl IDlar
}
