package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"convertpdfgo/pkg/logger"
	"convertpdfgo/storage"
)

type otpRepo struct {
	db  *pgxpool.Pool
	log logger.ILogger
}

func NewOTPRepo(db *pgxpool.Pool, log logger.ILogger) storage.IOTPStorage {
	return &otpRepo{
		db:  db,
		log: log,
	}
}

func (r *otpRepo) Create(ctx context.Context, email string, code string, expiresAt time.Time) (string, error) {
	id := uuid.New().String()

	query := `
		INSERT INTO otp (id, email, code, status, expires_at)
		VALUES ($1, $2, $3, 'unconfirmed', $4)
	`

	_, err := r.db.Exec(ctx, query, id, email, code, expiresAt)
	if err != nil {
		r.log.Error("error inserting OTP", logger.Error(err))
		return "", err
	}

	return id, nil
}

func (r *otpRepo) GetUnconfirmedByID(ctx context.Context, id string) (string, string, time.Time, error) {
	var (
		email     string
		code      string
		expiresAt time.Time
	)

	query := `
		SELECT email, code, expires_at
		FROM otp
		WHERE id = $1 AND status = 'unconfirmed'
	`

	err := r.db.QueryRow(ctx, query, id).Scan(&email, &code, &expiresAt)
	if err != nil {
		r.log.Error("error getting otp by id", logger.Error(err))
		return "", "", time.Time{}, err
	}

	return email, code, expiresAt, nil
}

func (r *otpRepo) UpdateStatusToConfirmed(ctx context.Context, id string) error {
	query := `
		UPDATE otp
		SET status = 'confirmed'
		WHERE id = $1
	`

	_, err := r.db.Exec(ctx, query, id)
	if err != nil {
		r.log.Error("error updating otp status", logger.Error(err))
		return err
	}

	return nil
}

func (r *otpRepo) GetByIDAndEmail(ctx context.Context, id string, email string) (bool, error) {
	query := `
		SELECT 1
		FROM otp
		WHERE id = $1 AND email = $2
	`

	var exists int
	err := r.db.QueryRow(ctx, query, id, email).Scan(&exists)
	if err != nil {
		return false, err
	}

	return true, nil
}
