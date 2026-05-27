// Contact-form service.
//
//	POST /api/contact            → Submit  (open; rate-limited globally)
//	GET  /admin/contacts         → List    (admin, contacts:read)
//	GET  /admin/contacts/:id     → Get     (admin)
//	POST /admin/contacts/:id/reply → Reply (admin, contacts:write)
//	                                 — also fires the email via Brevo
//	PUT  /admin/contacts/:id/status → UpdateStatus
//	DELETE /admin/contacts/:id   → Delete
package service

import (
	"context"
	"fmt"
	"strings"

	"convertpdfgo/api/models"
	"convertpdfgo/config"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/storage"
)

type ContactService interface {
	Submit(ctx context.Context, req models.ContactSubmitRequest, userID *string) (string, error)
	List(ctx context.Context, f models.ContactListFilter) ([]models.ContactMessage, int, error)
	Get(ctx context.Context, id string) (*models.ContactMessage, error)
	Reply(ctx context.Context, messageID, adminUserID string, req models.ContactReplyRequest) (replyID string, err error)
	UpdateStatus(ctx context.Context, id, status string) error
	Delete(ctx context.Context, id string) error
}

type contactService struct {
	stg    storage.IContactStorage
	log    logger.ILogger
	mailer MailerService
	cfg    config.Config
}

func NewContactService(stg storage.IStorage, log logger.ILogger, mailer MailerService, cfg config.Config) ContactService {
	return &contactService{
		stg:    stg.Contact(),
		log:    log,
		mailer: mailer,
		cfg:    cfg,
	}
}

func (s *contactService) Submit(ctx context.Context, req models.ContactSubmitRequest, userID *string) (string, error) {
	m := &models.ContactMessage{
		UserID: userID,
		Name:   strings.TrimSpace(req.Name),
		Email:  strings.TrimSpace(strings.ToLower(req.Email)),
		Body:   req.Message,
	}
	if err := s.stg.Submit(ctx, m); err != nil {
		return "", err
	}
	return m.ID, nil
}

func (s *contactService) List(ctx context.Context, f models.ContactListFilter) ([]models.ContactMessage, int, error) {
	return s.stg.List(ctx, f)
}

func (s *contactService) Get(ctx context.Context, id string) (*models.ContactMessage, error) {
	return s.stg.GetByID(ctx, id)
}

// Reply persists the admin reply, flips the parent message to
// 'replied' in the same transaction, then ships an email to the
// original sender via the existing Brevo mailer. Email failure is
// non-fatal: the reply row exists, the admin sees it in the thread,
// and a follow-up notice can be triggered manually if needed.
func (s *contactService) Reply(ctx context.Context, messageID, adminUserID string, req models.ContactReplyRequest) (string, error) {
	parent, err := s.stg.GetByID(ctx, messageID)
	if err != nil {
		return "", fmt.Errorf("get parent: %w", err)
	}

	replyID, err := s.stg.SaveReply(ctx, messageID, adminUserID, req.Body)
	if err != nil {
		return "", err
	}

	subject := parent.Subject
	if subject == "" {
		subject = "Your message to convertpdfgo"
	}
	if !strings.HasPrefix(strings.ToLower(subject), "re:") {
		subject = "Re: " + subject
	}

	body := buildReplyEmailBody(parent, req.Body)
	if err := s.mailer.Send(parent.Email, subject, body); err != nil {
		// Logged + swallowed: the admin still has a UI record of the
		// reply, and we don't want a transient SMTP outage to make
		// the reply look "unsaved". WEAKNESSES.md #22 silent-error
		// metric tracks rates if this becomes a pattern.
		s.log.Warning("contact reply: email send failed",
			logger.Error(err),
			logger.String("to", parent.Email),
			logger.String("message_id", messageID),
		)
	}
	return replyID, nil
}

func (s *contactService) UpdateStatus(ctx context.Context, id, status string) error {
	return s.stg.UpdateStatus(ctx, id, status)
}

func (s *contactService) Delete(ctx context.Context, id string) error {
	return s.stg.Delete(ctx, id)
}

// buildReplyEmailBody composes a plain-text body that includes the
// admin reply on top, followed by a quoted "you wrote" block so the
// user has context. Keeping it text-only mirrors the rest of our
// transactional mail (Brevo accepts both, plain text is fine for the
// "support response" flow).
func buildReplyEmailBody(parent *models.ContactMessage, replyBody string) string {
	var b strings.Builder
	b.WriteString("Hi ")
	b.WriteString(parent.Name)
	b.WriteString(",\n\n")
	b.WriteString(replyBody)
	b.WriteString("\n\n— convertpdfgo support\n\n")
	b.WriteString("--- Your original message ---\n")
	if parent.Subject != "" {
		b.WriteString("Subject: ")
		b.WriteString(parent.Subject)
		b.WriteString("\n")
	}
	b.WriteString(parent.Body)
	b.WriteString("\n")
	return b.String()
}
