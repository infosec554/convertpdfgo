package models

import "time"

// Permission rows live in the seeded `permissions` table. The `Name` field
// is the wire format used by RequirePermission ("users:read", etc.).
type Permission struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// UserListFilter narrows the admin user-list endpoint. Empty fields mean
// "no filter on this column". Limit / Offset paginate; Limit defaults to 50
// at the storage layer if 0.
type UserListFilter struct {
	Search string `form:"q"`      // matches email or name (ILIKE)
	Role   string `form:"role"`   // exact match: user | moderator | admin | superadmin
	Status string `form:"status"` // active | blocked | deleted
	Limit  int    `form:"limit"`
	Offset int    `form:"offset"`
}

type UpdateUserRoleRequest struct {
	Role string `json:"role" binding:"required,oneof=user moderator admin superadmin"`
}

type UpdateUserStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=active blocked deleted"`
}
