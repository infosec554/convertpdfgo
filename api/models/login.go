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
