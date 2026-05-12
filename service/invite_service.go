// Admin invite flow.
//
//	(1) POST /admin/users/invite  → InviteService.Create
//	    - inserts a status='pending' user row
//	    - mints a 32-hex-char token, stashes in Redis (TTL 48 h)
//	    - sends the invite email with the accept link
//	(2) GET  /auth/invite/:token  → InviteService.Preview
//	    - lets the accept page render "Invited as <role>" copy
//	      without leaking the user_id
//	(3) POST /auth/accept-invite  → InviteService.Accept
//	    - bcrypt the password, flip the user row to active
//	    - blow the token away from Redis so it's single-use
//	    - returns the new user's id + role so the handler can mint
//	      the access/refresh JWT pair just like /auth/login does
package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"convertpdfgo/api/models"
	"convertpdfgo/config"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/storage"
)

type InviteService interface {
	Create(ctx context.Context, req models.InviteAdminRequest, invitedBy string) (token string, err error)
	Preview(ctx context.Context, token string) (models.InvitePreview, error)
	Accept(ctx context.Context, req models.AcceptInviteRequest) (userID, role string, err error)
}

type inviteService struct {
	stg    storage.IStorage
	redis  storage.IRedisStorage
	log    logger.ILogger
	mailer MailerService
	cfg    config.Config
}

func NewInviteService(stg storage.IStorage, redis storage.IRedisStorage, log logger.ILogger, mailer MailerService, cfg config.Config) InviteService {
	return &inviteService{stg: stg, redis: redis, log: log, mailer: mailer, cfg: cfg}
}

func (s *inviteService) Create(ctx context.Context, req models.InviteAdminRequest, invitedBy string) (string, error) {
	email := strings.TrimSpace(strings.ToLower(req.Email))
	if email == "" {
		return "", fmt.Errorf("email required")
	}
	if req.Role != "user" && req.Role != "admin" {
		return "", fmt.Errorf("role must be user or admin")
	}

	userID, err := s.stg.User().CreatePending(ctx, email, req.Role)
	if err != nil {
		return "", fmt.Errorf("pending row: %w", err)
	}

	token, err := newInviteToken()
	if err != nil {
		return "", err
	}
	rec := models.InviteRecord{
		UserID:    userID,
		Email:     email,
		Role:      req.Role,
		InvitedBy: invitedBy,
		CreatedAt: time.Now(),
	}
	payload, _ := json.Marshal(rec)
	if err := s.redis.SetX(ctx, models.InviteRedisPrefix+token, string(payload), models.InviteTTL); err != nil {
		return "", fmt.Errorf("redis: %w", err)
	}

	// Best-effort email. If the mailer is mis-configured we still
	// return the token to the caller — they can copy/paste the link
	// to the invitee out of band. The error is logged so an admin
	// notices.
	link := s.cfg.AppURL + "/admin/accept?token=" + token
	subject := "You've been invited as an admin to convertpdfgo"
	body := fmt.Sprintf(
		"Hi,\n\n"+
			"You've been invited to convertpdfgo with the role: %s\n\n"+
			"Set your password and finish signing in here:\n  %s\n\n"+
			"This link is valid for 48 hours.\n",
		req.Role, link,
	)
	if err := s.mailer.Send(email, subject, body); err != nil {
		s.log.Warning("invite: email send failed", logger.Error(err), logger.String("email", email))
	}
	return token, nil
}

func (s *inviteService) Preview(ctx context.Context, token string) (models.InvitePreview, error) {
	rec, err := s.loadInvite(ctx, token)
	if err != nil {
		return models.InvitePreview{}, err
	}
	return models.InvitePreview{Email: rec.Email, Role: rec.Role}, nil
}

func (s *inviteService) Accept(ctx context.Context, req models.AcceptInviteRequest) (string, string, error) {
	rec, err := s.loadInvite(ctx, req.Token)
	if err != nil {
		return "", "", err
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return "", "", fmt.Errorf("hash: %w", err)
	}
	if err := s.stg.User().ActivatePending(ctx, rec.UserID, string(hashed)); err != nil {
		return "", "", fmt.Errorf("activate: %w", err)
	}
	// Single-use: drop the token so the link can't be replayed.
	if err := s.redis.Del(ctx, models.InviteRedisPrefix+req.Token); err != nil {
		s.log.Warning("invite: token cleanup", logger.Error(err))
	}
	return rec.UserID, rec.Role, nil
}

// loadInvite pulls the invite record from Redis. Returns a clear
// "expired or invalid" error rather than leaking pgx specifics.
func (s *inviteService) loadInvite(ctx context.Context, token string) (models.InviteRecord, error) {
	if token == "" {
		return models.InviteRecord{}, fmt.Errorf("token required")
	}
	raw, err := s.redis.Get(ctx, models.InviteRedisPrefix+token)
	if err != nil || raw == "" {
		return models.InviteRecord{}, fmt.Errorf("invite expired or invalid")
	}
	var rec models.InviteRecord
	if err := json.Unmarshal([]byte(raw), &rec); err != nil {
		return models.InviteRecord{}, fmt.Errorf("decode invite: %w", err)
	}
	return rec, nil
}

// newInviteToken returns 32 hex chars (16 random bytes). That's
// enough entropy that the URL-safe ID is unguessable in any practical
// timeframe and short enough to fit comfortably in an email line.
func newInviteToken() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
