package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/storage"
)

type roleRepo struct {
	db  *pgxpool.Pool
	log logger.ILogger
}

func NewRoleRepo(db *pgxpool.Pool, log logger.ILogger) storage.IRoleStorage {
	return &roleRepo{
		db:  db,
		log: log,
	}
}

func (r *roleRepo) Create(ctx context.Context, name string, createdBy string) (string, error) {
	id := uuid.New().String()
	query := `
		INSERT INTO roles(id, name, status, created_by)
		VALUES ($1, $2, 'active', $3)
	`
	_, err := r.db.Exec(ctx, query, id, name, createdBy)
	if err != nil {
		r.log.Error("error inserting role", logger.Error(err))
		return "", err
	}
	return id, nil
}

func (r *roleRepo) Update(ctx context.Context, id string, name string) error {
	query := `
		UPDATE roles
		SET name = $1
		WHERE id = $2
	`
	_, err := r.db.Exec(ctx, query, name, id)
	if err != nil {
		r.log.Error("error updating role", logger.Error(err))
	}
	return err
}

func (r *roleRepo) GetAll(ctx context.Context) ([]models.Role, error) {
	query := `
		SELECT id, name, created_at
		FROM roles
		WHERE status = 'active'
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		r.log.Error("error querying roles", logger.Error(err))
		return nil, err
	}
	defer rows.Close()

	var roles []models.Role

	for rows.Next() {
		var role models.Role
		if err := rows.Scan(&role.ID, &role.Name, &role.CreatedAt); err != nil {
			r.log.Error("error scanning role", logger.Error(err)) // ✅ bu yerda r - roleRepo
			continue
		}
		roles = append(roles, role)
	}

	return roles, nil
}

func (r *roleRepo) Exists(ctx context.Context, id string) (bool, error) {
	query := `
		SELECT COUNT(1)
		FROM roles
		WHERE id = $1 AND status = 'active'
	`
	var count int
	err := r.db.QueryRow(ctx, query, id).Scan(&count)
	if err != nil {
		r.log.Error("error checking role existence", logger.Error(err))
		return false, err
	}
	return count > 0, nil
}
