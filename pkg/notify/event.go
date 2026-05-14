// Package notify carries fire-and-forget events from the API and worker
// processes to anything that wants to react in real time — currently just
// the in-process WebSocket Hub, but the Bus interface deliberately leaves
// room for a Redis pub/sub backplane the day we run more than one API
// replica.
//
// Two audiences:
//
//   - User-scoped events go to the owning user's WS clients (e.g. job done).
//   - Admin events fan out to every connected admin (e.g. new signup).
//
// Events are JSON-serialised on the wire so the same Envelope can flow
// through Redis later without code changes on either side.
package notify

import (
	"encoding/json"
	"time"
)

// Type names the event for the JS client's switch statement. Keep these
// stable — clients pin to them.
type Type string

const (
	// User-scoped:
	TypeJobUpdate Type = "job.update" // status moved (queued / processing / done / failed)

	// Admin-broadcast:
	TypeUserRegistered Type = "admin.user.registered"
	TypeContactNew     Type = "admin.contact.new"
	TypeSuspiciousAuth Type = "admin.auth.suspicious"
	TypeJobFailed      Type = "admin.job.failed"
)

// Envelope is the wire format. Data is whatever payload struct the Type
// implies; clients dispatch on Type and JSON.parse the Data field.
type Envelope struct {
	Type      Type            `json:"type"`
	Timestamp time.Time       `json:"ts"`
	Data      json.RawMessage `json:"data"`
}

// JobUpdate is the payload for TypeJobUpdate. UserID is the owner;
// guests have empty UserID and never receive events.
type JobUpdate struct {
	UserID       string `json:"user_id"`
	JobID        string `json:"job_id"`
	JobType      string `json:"job_type"` // "merge", "compress", ... (matches queue task name suffix)
	Status       string `json:"status"`   // "queued" | "processing" | "done" | "failed"
	OutputFileID string `json:"output_file_id,omitempty"`
	Error        string `json:"error,omitempty"`
}

// UserRegistered is the payload for TypeUserRegistered.
type UserRegistered struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
}

// ContactNew is the payload for TypeContactNew (a user submitted the
// contact form).
type ContactNew struct {
	ContactID string `json:"contact_id"`
	Email     string `json:"email"`
	Subject   string `json:"subject"`
}

// SuspiciousAuth fires when login_limiter trips (5 failed attempts /
// 10 min) so an admin sees the brute-force in real time.
type SuspiciousAuth struct {
	Email     string `json:"email"`
	IP        string `json:"ip"`
	UserAgent string `json:"user_agent,omitempty"`
}

// JobFailed mirrors the user-side failure to admins so they can spot
// systemic worker issues without trawling the audit log.
type JobFailed struct {
	JobID   string `json:"job_id"`
	JobType string `json:"job_type"`
	Error   string `json:"error"`
}

// NewEnvelope marshals the data into an Envelope at the current time.
// Marshal failures are vanishingly rare on our payloads (no maps with
// non-string keys, no channels) — we return the error so the caller
// can log it but most call sites can ignore it.
func NewEnvelope(t Type, data any) (Envelope, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return Envelope{}, err
	}
	return Envelope{Type: t, Timestamp: time.Now().UTC(), Data: raw}, nil
}
