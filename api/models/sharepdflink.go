package models

import "time"

type CreateSharedLinkRequest struct {
	FileID    string     `json:"file_id" binding:"required"`
	ExpiresAt *time.Time `json:"expires_at,omitempty" example:"2025-07-27T00:00:00Z"` // *pointer bo‘lishi kerak!
}

type SharedLink struct {
	ID          string    `json:"id"`
	FileID      string    `json:"file_id"`
	SharedToken string    `json:"shared_token"`
	ExpiresAt   time.Time `json:"expires_at,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}
