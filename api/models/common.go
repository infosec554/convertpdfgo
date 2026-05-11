package models

type PrimaryKey struct {
	ID string `json:"id"`
}

// JobCreatedResponse is the body every "create a job" endpoint returns.
// Single shape across the surface so frontend clients can share one
// type for poll-driven flows.
//
// Status is "pending" for async endpoints (which return 202) and
// "done" for the few synchronous ones like /api/pdf/inspect (200).
type JobCreatedResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// JobStatus values used across job rows + the JobCreatedResponse Status
// field, so handlers and tests share one source of truth.
const (
	JobStatusPending    = "pending"
	JobStatusProcessing = "processing"
	JobStatusDone       = "done"
	JobStatusFailed     = "failed"
)
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type RefreshTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}
