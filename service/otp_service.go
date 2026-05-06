package service

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"time"

	"convertpdfgo/pkg/logger"
	"convertpdfgo/pkg/mailer"
	"convertpdfgo/storage"
)

type OtpService interface {
	SendOtp(ctx context.Context, email string) (string, error)
	GetUnconfirmedByID(ctx context.Context, id string) (string, string, time.Time, error)
	UpdateStatusToConfirmed(ctx context.Context, id string) error
	GetByIDAndEmail(ctx context.Context, id, email string) (bool, error)
}

type otpService struct {
	stg    storage.IOTPStorage
	log    logger.ILogger
	mailer mailer.IMailer
	redis  storage.IRedisStorage
}

func NewOtpService(stg storage.IStorage, log logger.ILogger, m mailer.IMailer, redis storage.IRedisStorage) OtpService {
	return &otpService{stg: stg.OTP(), log: log, mailer: m, redis: redis}
}

func (s *otpService) SendOtp(ctx context.Context, email string) (string, error) {
	code := generateCode()
	expiresAt := time.Now().Add(5 * time.Minute)

	id, err := s.stg.Create(ctx, email, code, expiresAt)
	if err != nil {
		s.log.Error("otp: create", logger.Error(err), logger.String("email", email))
		return "", err
	}

	if err := s.redis.SetX(ctx, "otp:"+id, code, 5*time.Minute); err != nil {
		s.log.Error("otp: redis set", logger.Error(err))
	}

	body, err := mailer.RenderOTPEmail(mailer.OTPEmailData{Code: code})
	if err != nil {
		s.log.Error("otp: render template", logger.Error(err))
		return "", err
	}
	if err := s.mailer.Send(email, "Your OTP Code", body); err != nil {
		s.log.Error("otp: mailer send", logger.Error(err), logger.String("email", email))
		return "", err
	}

	s.log.Info("otp sent", logger.String("otp_id", id), logger.String("email", email))
	return id, nil
}

func (s *otpService) GetUnconfirmedByID(ctx context.Context, id string) (string, string, time.Time, error) {
	if val, err := s.redis.Get(ctx, "otp:"+id); err == nil && val != "" {
		email, _, expiresAt, dberr := s.stg.GetUnconfirmedByID(ctx, id)
		if dberr != nil {
			s.log.Error("otp: db fallback", logger.Error(dberr))
			return "", "", time.Time{}, dberr
		}
		return email, val, expiresAt, nil
	}
	return s.stg.GetUnconfirmedByID(ctx, id)
}

func (s *otpService) UpdateStatusToConfirmed(ctx context.Context, id string) error {
	return s.stg.UpdateStatusToConfirmed(ctx, id)
}

func (s *otpService) GetByIDAndEmail(ctx context.Context, id, email string) (bool, error) {
	return s.stg.GetByIDAndEmail(ctx, id, email)
}

// generateCode returns a cryptographically random 6-digit code. The previous
// implementation used time.Now().UnixNano() % 1e6 which made codes mostly
// monotonic and trivially guessable.
func generateCode() string {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand on Linux only fails if the kernel is broken; degrade
		// to time-based as a last resort.
		return fmt.Sprintf("%06d", time.Now().UnixNano()%1000000)
	}
	n := binary.BigEndian.Uint32(b[:]) % 1000000
	return fmt.Sprintf("%06d", n)
}
