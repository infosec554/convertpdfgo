package models

type InspectRequest struct {
	FileID string `json:"file_id" binding:"required" example:"1e2b3c4d-5f6a-7b89-cd01-23456789abcd"`
}

type InspectJob struct {
	ID        string `json:"id" example:"d4f56987-e123-4567-abcd-7890abcdef12"`
	UserID    string `json:"user_id"`
	FileID    string `json:"file_id"`
	PageCount int    `json:"page_count" example:"5"`
	Title     string `json:"title" example:"My PDF Document"`
	Author    string `json:"author" example:"John Doe"`
	Subject   string `json:"subject" example:"Report"`
	Keywords  string `json:"keywords" example:"report,2025"`
	Status    string `json:"status" example:"done"`
	CreatedAt string `json:"created_at"`
}
