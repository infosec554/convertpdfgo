package models

import "time"

type User struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Status    string    `json:"status"`
	Role      string    `json:"role"` // user | moderator | admin | superadmin
	CreatedAt time.Time `json:"created_at"`
	CreatedBy string    `json:"created_by"`
}

type GetUserByEmailRequest struct {
	Email string `json:"email"`
}

type GetUserByEmailResponse struct {
	ID       string `json:"id"`
	Password string `json:"password"`
	Status   string `json:"status"`
}

type CreateUser struct {
	Name     string `json:"name"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password,omitempty"`
	Provider string `json:"auth_provider,omitempty"` // email, google, github
}

type OAuthLoginRequest struct {
	Provider string `json:"provider" binding:"required"`
	Token    string `json:"token" binding:"required"`
}
