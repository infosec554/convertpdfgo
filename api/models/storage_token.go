package models

import "time"

// Provider name constants. Treat as the canonical identifier in URLs
// (/oauth/{provider}/...) and in the user_storage_tokens.provider
// column.
const (
	ProviderDropbox  = "dropbox"
	ProviderGDrive   = "gdrive"
	ProviderOneDrive = "onedrive"
)

// UserStorageToken is one row in user_storage_tokens — the OAuth
// credentials we hold on behalf of a user for a specific provider.
// Same shape across providers; the differences are encapsulated in
// pkg/storage_provider.
type UserStorageToken struct {
	ID            string     `json:"id"`
	UserID        string     `json:"user_id"`
	Provider      string     `json:"provider"`
	AccessToken   string     `json:"-"` // never serialised to clients
	RefreshToken  string     `json:"-"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	Scope         string     `json:"scope,omitempty"`
	AccountEmail  string     `json:"account_email,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// StorageProviderInfo is the public-facing description of one export
// target, returned by GET /api/storage/providers.
type StorageProviderInfo struct {
	Name        string `json:"name"`         // "dropbox", "gdrive", "onedrive"
	DisplayName string `json:"display_name"` // "Dropbox", "Google Drive", "OneDrive"
	Configured  bool   `json:"configured"`   // false → UI hides this option
	Connected   bool   `json:"connected"`    // current user already authed
	AccountEmail string `json:"account_email,omitempty"`
}

// ExportFileRequest is the body of POST /file/{id}/export.
type ExportFileRequest struct {
	Provider string `json:"provider" binding:"required"` // dropbox | gdrive | onedrive
	Path     string `json:"path"`                        // remote path/filename; default = original name
}

// ExportFileResponse is what the export endpoint returns on success.
type ExportFileResponse struct {
	Provider   string `json:"provider"`
	RemotePath string `json:"remote_path"`
	// SharedURL is the publicly viewable URL the provider returned, if
	// it gave us one. Dropbox issues these on demand via
	// sharing/create_shared_link_with_settings; GDrive needs a
	// follow-up permissions.create call. Empty when unavailable.
	SharedURL string `json:"shared_url,omitempty"`
}
