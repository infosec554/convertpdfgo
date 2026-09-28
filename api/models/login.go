package models

// LoginUser is returned by user_service.GetForLoginByEmail(). The OAuth
// completion path uses it to decide whether an inbound social profile
// matches an existing local user. The Password field is preserved for
// schema compatibility but is no longer consulted now that the
// email+password login route has been removed.
type LoginUser struct {
	ID       string `json:"id"`
	Password string `json:"password"`
	Status   string `json:"status"`
	Role     string `json:"role"` // user | admin
}

// LoginRequest is the body for POST /auth/login (email + password). Used by
// the admin console, whose operators authenticate with a local password
// rather than an OAuth identity provider.
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// ChangePasswordRequest is the body for PUT /me/password. The caller proves
// ownership with their current password before the new one is accepted.
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=8"`
}
