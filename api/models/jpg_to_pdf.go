package models

import "time"

type JPGToPDFJob struct {
	ID           string    `json:"id"`
	UserID       *string   `json:"user_id"`
	InputFileIDs []string  `json:"input_file_ids"`
	OutputFileID *string   `json:"output_file_id,omitempty"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}
// CreateJPGToPDFRequest – bir nechta JPG/PNG faylni bitta PDF ga jamlash.
// 1..50 ta fayl — image batch'i merge bilan bir xil chegarada (50).
type CreateJPGToPDFRequest struct {
	InputFileIDs []string `json:"input_file_ids" binding:"required,min=1,max=50,dive,uuid"`
}
