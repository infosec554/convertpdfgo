package models

import "time"

type RemovePagesRequest struct {
	// Bindings match the other PDF-op request models: both fields are
	// mandatory, and InputFileID must parse as a UUID. The previous
	// schema let an empty body through, which then surfaced as a 500
	// when the storage layer tried to insert "" into a uuid column.
	InputFileID   string `json:"input_file_id"   binding:"required,uuid"`
	PagesToRemove string `json:"pages_to_remove" binding:"required"` // e.g. "1,4-6"
}

type RemoveJob struct {
	ID            string    `json:"id"`
	UserID        *string   `json:"user_id"` // NULL bo‘lishi mumkin
	InputFileID   string    `json:"input_file_id"`
	PagesToRemove string    `json:"pages_to_remove"`
	OutputFileID  *string   `json:"output_file_id"` // NULL bo‘lishi mumkin
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}
