package models

import "time"

// --- DB rows -------------------------------------------------------------

type ContactMessage struct {
	ID         string     `json:"id"`
	UserID     *string    `json:"user_id,omitempty"` // nil for guest submissions
	Name       string     `json:"name"`
	Email      string     `json:"email"`
	Subject    string     `json:"subject,omitempty"`
	Body       string     `json:"body"`
	Status     string     `json:"status"` // new | replied | closed
	RepliedAt  *time.Time `json:"replied_at,omitempty"`
	RepliedBy  *string    `json:"replied_by,omitempty"` // admin user_id
	CreatedAt  time.Time  `json:"created_at"`

	// Replies is populated only by ContactStorage.GetByID — list
	// endpoints leave it nil to keep payloads small.
	Replies []ContactReply `json:"replies,omitempty"`
}

type ContactReply struct {
	ID                string    `json:"id"`
	ContactMessageID  string    `json:"contact_message_id"`
	AdminUserID       string    `json:"admin_user_id"`
	Body              string    `json:"body"`
	SentAt            time.Time `json:"sent_at"`
}

// --- Request / filter shapes --------------------------------------------

// ContactSubmitRequest is the body for POST /api/contact. Email is
// required so the admin reply has somewhere to land — even when the
// caller is logged in (we don't infer it from JWT silently; let the
// user confirm where the reply goes).
type ContactSubmitRequest struct {
	Name    string `json:"name"    binding:"required,min=2,max=255"`
	Email   string `json:"email"   binding:"required,email"`
	Subject string `json:"subject" binding:"max=255"`
	Body    string `json:"body"    binding:"required,min=5,max=8000"`
}

// ContactListFilter — admin dashboard query for GET /admin/contacts.
// Empty fields mean "any". Limit defaults to 50 at storage, hard-
// capped at 200 to keep responses bounded.
type ContactListFilter struct {
	Status string `form:"status" json:"-"` // new | replied | closed | "" (any)
	Q      string `form:"q"      json:"-"` // ILIKE on email / name / subject
	Limit  int    `form:"limit"  json:"-"`
	Offset int    `form:"offset" json:"-"`
}

// ContactReplyRequest — admin replying to a message. The handler
// emails Body to the message's email via the existing Brevo mailer
// and records a row in contact_replies.
type ContactReplyRequest struct {
	Body string `json:"body" binding:"required,min=5,max=20000"`
}

// ContactUpdateStatusRequest — admin marking a message as
// replied/closed without sending an email (e.g. handled out of band).
type ContactUpdateStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=new replied closed"`
}
