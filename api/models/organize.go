package models

import "time"


type OrganizeJob struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	InputFileID  string    `json:"input_file_id"`
	NewOrder     []int     `json:"new_order"`      
	OutputFileID string    `json:"output_file_id"`
	Status       string    `json:"status"`         
	CreatedAt    time.Time `json:"created_at"`    
}

type CreateOrganizeJobRequest struct {
	InputFileID string `json:"input_file_id" example:"f680eafd-4c56-435d-8a3b-60f0d15b7f4e"`
	NewOrder    []int  `json:"new_order" example:"[3,1,2]"` 
}
