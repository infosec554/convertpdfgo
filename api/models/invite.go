package models

import "time"

// InviteAdminRequest is the body for POST /admin/users/invite.
// Role is locked to "admin" or "user" per the product decision —
// no moderator / superadmin in the system anymore. Frontend should
// still validate; the handler re-validates server-side.
type InviteAdminRequest struct {
	Email string `json:"email" binding:"required,email"`
	Role  string `json:"role"  binding:"required,oneof=user admin"`
}

// InviteRecord is what we stash in Redis under invite:<token>. The
// token itself isn't repeated in the value (it's already the key)
// so /auth/accept-invite only needs one lookup. CreatedAt is kept
// for human-readable expiry math on the accept screen.
type InviteRecord struct {
	UserID    string    `json:"user_id"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	InvitedBy string    `json:"invited_by"`
	CreatedAt time.Time `json:"created_at"`
}

// AcceptInviteRequest is POST /auth/accept-invite. The Token comes
// from the email link; Password is the value the user typed into
// the "set password" form.
type AcceptInviteRequest struct {
	Token    string `json:"token"    binding:"required"`
	Password string `json:"password" binding:"required,min=8"`
}

// InvitePreview is the unauthenticated /auth/invite/:token response —
// just enough to render the "you've been invited as <role>" screen
// without leaking which user_id the token resolves to.
type InvitePreview struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

const (
	// InviteTTL is how long an invite link stays valid. 48 hours is
	// the SaaS default — long enough that the new admin's email gets
	// read, short enough that a leaked token doesn't sit forever.
	InviteTTL = 48 * time.Hour

	// invite:<token> Redis key prefix. Lives in the model package so
	// service + handler use the same constant without a dependency
	// edge between them.
	InviteRedisPrefix = "invite:"
)
