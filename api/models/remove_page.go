package models

import "time"

type RemovePagesRequest struct {
	InputFileID   string `json:"input_file_id"`
	PagesToRemove string `json:"pages_to_remove"` // e.g. "1,4-6"
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
