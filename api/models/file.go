package models

import "time"

type File struct {
	ID         string    `json:"id"          db:"id"`
	UserID     *string   `json:"user_id"     db:"user_id"` // NULL bo‘lishi mumkin
	FileName   string    `json:"file_name"   db:"file_name"`
	FilePath   string    `json:"file_path"   db:"file_path"`
	FileType   string    `json:"file_type"   db:"file_type"`
	FileSize   int64     `json:"file_size"   db:"file_size"`
	SHA256     *string   `json:"sha256"      db:"sha256"` // hex-encoded; nil for legacy rows
	UploadedAt time.Time `json:"uploaded_at" db:"uploaded_at"`
}

type OldFile struct {
	ID       string `json:"id"`
	FilePath string `json:"file_path"`
}
