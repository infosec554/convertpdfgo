package models

type CreateRole struct {
	Name string `json:"name" binding:"required"`
}

type UpdateRole struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Role struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

type RoleListResponse struct {
	Roles []Role `json:"roles"`
}
