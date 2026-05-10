package models

type Log struct {
	ID        string `json:"id"`
	JobID     string `json:"job_id"`
	JobType   string `json:"job_type"`
	Message   string `json:"message"`
	Level     string `json:"level"`
	CreatedAt string `json:"created_at"`
}
