package models

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
	UserType string `json:"user_type"`
}

type LoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}
type LoginUser struct {
	ID       string `json:"id"`
	Password string `json:"password"`
	Status   string `json:"status"`
	Role     string `json:"role"` // user | moderator | admin | superadmin
}
