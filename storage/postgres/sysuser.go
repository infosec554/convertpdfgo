package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"convertpdfgo/pkg/logger"
	"convertpdfgo/storage"
)

type sysuserRepo struct {
	db  *pgxpool.Pool
	log logger.ILogger
}

func NewSysuserRepo(db *pgxpool.Pool, log logger.ILogger) storage.ISysuserStorage {
	return &sysuserRepo{
		db:  db,
		log: log,
	}
}

func (r *sysuserRepo) GetByPhone(ctx context.Context, phone string) (id, hashedPassword, status string, err error) {
	query := `
		SELECT id, password, status
		FROM sysusers
		WHERE phone = $1 AND status IN ('active', 'blocked')
	`

	err = r.db.QueryRow(ctx, query).Scan(&id, &hashedPassword, &status)
	if err != nil {
		r.log.Error("error getting sysuser by phone", logger.Error(err))
	}

	return
}

func (r *sysuserRepo) Create(ctx context.Context, name, phone, hashedPassword, createdBy string) (string, error) {
	id := uuid.New().String()

	query := `
		INSERT INTO sysusers (id, name, phone, password, status, created_by)
		VALUES ($1, $2, $3, $4, 'active', $5)
	`

	_, err := r.db.Exec(ctx, query, id, name, phone, hashedPassword, createdBy)
	if err != nil {
		r.log.Error("error inserting sysuser", logger.Error(err))
		return "", err
	}

	return id, nil
}

func (r *sysuserRepo) AssignRoles(ctx context.Context, sysuserID string, roleIDs []string) error {
	for _, roleID := range roleIDs {
		id := uuid.New().String()

		query := `
			INSERT INTO sysuser_roles (id, sysuser_id, role_id)
			VALUES ($1, $2, $3)
		`

		_, err := r.db.Exec(ctx, query, id, sysuserID, roleID)
		if err != nil {
			r.log.Error("error assigning role to sysuser", logger.Error(err))
			return err
		}
	}

	return nil
}
