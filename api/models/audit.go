package models

import "time"

const (
	AuthEventLogout     = "logout"
	AuthEventOAuthLogin = "oauth_login"
)

type AuthAudit struct {
	ID        string    `json:"id"`
	UserID    *string   `json:"user_id,omitempty"`
	Email     string    `json:"email,omitempty"`
	Event     string    `json:"event"`
	Provider  string    `json:"provider,omitempty"`
	IP        string    `json:"ip,omitempty"`
	UserAgent string    `json:"user_agent,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
