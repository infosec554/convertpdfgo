// Package storage_provider abstracts the "Save to ..." cloud
// destinations (Dropbox / Google Drive / OneDrive).
//
// Each Provider is a thin Go client over the provider's REST API:
//
//   - AuthURL builds the OAuth2 redirect URL the browser is sent to.
//   - Exchange swaps a one-time `code` for a Token after the user
//     returns from the provider's consent screen.
//   - Refresh trades a refresh_token for a fresh access_token when
//     the stored one has expired.
//   - Upload pushes one file into the user's account at the given
//     remote path, returning a Result with the canonical remote path
//     and (when the provider offers one) a shareable URL.
//
// The four-method surface keeps each provider implementation under
// ~120 LOC. Provider-specific behaviour (pagination, large-file
// chunking, scope strings) lives inside the implementations; callers
// see only the interface.
package storage_provider

import (
	"context"
	"io"
	"time"
)

// Token carries OAuth credentials in a provider-neutral shape. Empty
// RefreshToken means the provider doesn't issue one (some Dropbox
// scopes); empty Expiry means "no known expiry" (treat as long-lived).
type Token struct {
	AccessToken  string
	RefreshToken string
	Expiry       time.Time
	Scope        string
	// AccountEmail is filled when the provider's /me-style endpoint
	// is cheap enough to call during Exchange (Dropbox: yes, Google:
	// via the userinfo endpoint, OneDrive: yes via /me).
	AccountEmail string
}

// Expired reports whether the token is past its expiry. Treats a
// zero-value Expiry as "never expires" — the provider didn't
// advertise one, so we just keep using it until a 401 forces a
// refresh.
func (t Token) Expired() bool {
	if t.Expiry.IsZero() {
		return false
	}
	// Leave a 60-second cushion so we refresh before the request goes
	// out, not after we've already eaten the 401.
	return time.Now().Add(60 * time.Second).After(t.Expiry)
}

// UploadResult is what Provider.Upload returns on success.
type UploadResult struct {
	RemotePath string // canonical path as the provider stored it
	SharedURL  string // public link, when the provider offered one
}

// Provider is the four-method contract every cloud target implements.
type Provider interface {
	// Name returns the short identifier we use in URLs and DB rows.
	Name() string

	// AuthURL returns the URL to redirect the browser to so the user
	// can grant our app access. `state` is opaque — the OAuth handler
	// generates a CSRF token, stores it in Redis, and verifies on
	// callback.
	AuthURL(state string) string

	// Exchange converts a one-time code from the callback into a
	// Token. The redirect URL must match exactly what AuthURL used
	// (the provider checks it), so we keep that responsibility inside
	// the implementation.
	Exchange(ctx context.Context, code string) (Token, error)

	// Refresh trades a refresh_token for a new access_token. Returns
	// the new Token (the refresh_token may or may not change — keep
	// the old one if the provider doesn't return a new one).
	Refresh(ctx context.Context, refreshToken string) (Token, error)

	// Upload writes `data` to remotePath inside the authenticated
	// user's account. contentType is passed to the provider; size is
	// used by providers that need a Content-Length up front (most do).
	// `shared` requests a public link be created and returned in
	// UploadResult.SharedURL — silently no-op'd by providers that
	// require a follow-up admin action to allow sharing.
	Upload(ctx context.Context, token Token, remotePath, contentType string,
		data io.Reader, size int64, shared bool) (UploadResult, error)
}
