package models

import "time"

// CreateAddHeaderFooterRequest – foydalanuvchidan keladigan so‘rov
type CreateAddHeaderFooterRequest struct {
	InputFileID string `json:"input_file_id" binding:"required"`

	HeaderText string `json:"header_text,omitempty"`
	FooterText string `json:"footer_text,omitempty"`

	FontSize  int    `json:"font_size,omitempty"`  // Agar 0 bo‘lsa, default 12 deb qaraladi
	FontColor string `json:"font_color,omitempty"` // Agar bo‘lmasa, "black" deb olinadi
	Position  string `json:"position,omitempty"`   // "left", "center", "right" (default "center")
}

// AddHeaderFooterJob – header/footer qo‘shish ishining modeli
type AddHeaderFooterJob struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	InputFileID  string    `json:"input_file_id"`
	HeaderText   string    `json:"header_text,omitempty"`
	FooterText   string    `json:"footer_text,omitempty"`
	FontSize     int       `json:"font_size"`
	FontColor    string    `json:"font_color"`
	Position     string    `json:"position"`
	OutputFileID string    `json:"output_file_id,omitempty"`
	Status       string    `json:"status"` // pending, done, failed
	CreatedAt    time.Time `json:"created_at"`
}
