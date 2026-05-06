package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/storage"
)

type userRepo struct {
	db  *pgxpool.Pool
	log logger.ILogger
}

func NewUserRepo(db *pgxpool.Pool, log logger.ILogger) storage.IUserStorage {
	return &userRepo{db: db, log: log}
}

func (r *userRepo) Create(ctx context.Context, req models.CreateUser) (string, error) {
	id := uuid.NewString()

	provider := req.Provider
	if provider == "" {
		provider = "email"
	}

	var pwd *string
	if req.Password != "" {
		pwd = &req.Password
	}
	var name *string
	if req.Name != "" {
		name = &req.Name
	}

	const q = `
		INSERT INTO users (id, name, email, password, auth_provider, status)
		VALUES ($1, $2, $3, $4, $5, 'active')
	`
	if _, err := r.db.Exec(ctx, q, id, name, req.Email, pwd, provider); err != nil {
		r.log.Error("user repo: insert", logger.Error(err))
		return "", err
	}
	return id, nil
}

func (r *userRepo) GetForLoginByEmail(ctx context.Context, email string) (models.LoginUser, error) {
	const q = `
		SELECT id, COALESCE(password, ''), status, COALESCE(role, 'user')
		FROM users
		WHERE email = $1 AND status = 'active'
	`
	var u models.LoginUser
	if err := r.db.QueryRow(ctx, q, email).Scan(&u.ID, &u.Password, &u.Status, &u.Role); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.LoginUser{}, err
		}
		r.log.Error("user repo: get by email", logger.Error(err))
		return models.LoginUser{}, err
	}
	return u, nil
}

func (r *userRepo) GetByID(ctx context.Context, id string) (*models.User, error) {
	const q = `
		SELECT id, COALESCE(name, ''), email, status, COALESCE(role, 'user'), created_at
		FROM users
		WHERE id = $1 AND status = 'active'
	`
	var u models.User
	if err := r.db.QueryRow(ctx, q, id).Scan(&u.ID, &u.Name, &u.Email, &u.Status, &u.Role, &u.CreatedAt); err != nil {
		r.log.Error("user repo: get by id", logger.Error(err))
		return nil, err
	}
	return &u, nil
}

func (r *userRepo) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	var exists bool
	const q = `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1 AND status = 'active')`
	if err := r.db.QueryRow(ctx, q, email).Scan(&exists); err != nil {
		r.log.Error("user repo: exists by email", logger.Error(err))
		return false, err
	}
	return exists, nil
}

func (r *userRepo) UpdatePassword(ctx context.Context, id, hashedPassword string) error {
	const q = `UPDATE users SET password = $2 WHERE id = $1 AND status = 'active'`
	tag, err := r.db.Exec(ctx, q, id, hashedPassword)
	if err != nil {
		r.log.Error("user repo: update password", logger.Error(err))
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *userRepo) UpdateProfile(ctx context.Context, id, name string) error {
	const q = `UPDATE users SET name = $2 WHERE id = $1 AND status = 'active'`
	tag, err := r.db.Exec(ctx, q, id, name)
	if err != nil {
		r.log.Error("user repo: update profile", logger.Error(err))
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// UpdateRole flips the user's role string. Allowed values are not enforced
// at the DB layer — that's the service / handler's job.
func (r *userRepo) UpdateRole(ctx context.Context, id, role string) error {
	const q = `UPDATE users SET role = $2 WHERE id = $1`
	tag, err := r.db.Exec(ctx, q, id, role)
	if err != nil {
		r.log.Error("user repo: update role", logger.Error(err))
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// UpdateStatus toggles active / blocked / deleted (we soft-delete here so
// audit & job rows remain queryable by user_id).
func (r *userRepo) UpdateStatus(ctx context.Context, id, status string) error {
	const q = `UPDATE users SET status = $2 WHERE id = $1`
	tag, err := r.db.Exec(ctx, q, id, status)
	if err != nil {
		r.log.Error("user repo: update status", logger.Error(err))
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// List returns paginated users matching the filter, plus the total count
// (so the UI can render "showing 1-50 of 1234").
func (r *userRepo) List(ctx context.Context, f models.UserListFilter) ([]models.User, int, error) {
	limit := f.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}

	const q = `
		SELECT id, COALESCE(name,''), email, status, COALESCE(role,'user'), created_at,
		       COUNT(*) OVER() AS total
		FROM users
		WHERE ($1 = '' OR email ILIKE '%' || $1 || '%' OR COALESCE(name,'') ILIKE '%' || $1 || '%')
		  AND ($2 = '' OR role = $2)
		  AND ($3 = '' OR status = $3)
		ORDER BY created_at DESC
		LIMIT $4 OFFSET $5
	`
	rows, err := r.db.Query(ctx, q, f.Search, f.Role, f.Status, limit, offset)
	if err != nil {
		r.log.Error("user repo: list", logger.Error(err))
		return nil, 0, err
	}
	defer rows.Close()

	var (
		out   []models.User
		total int
	)
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.Status, &u.Role, &u.CreatedAt, &total); err != nil {
			r.log.Error("user repo: list scan", logger.Error(err))
			continue
		}
		out = append(out, u)
	}
	return out, total, nil
}
