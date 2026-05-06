package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/storage"
)

type permissionRepo struct {
	db  *pgxpool.Pool
	log logger.ILogger
}

func NewPermissionRepo(db *pgxpool.Pool, log logger.ILogger) storage.IPermissionStorage {
	return &permissionRepo{db: db, log: log}
}

// ListPermissionsForRole returns the wire-format permission strings granted
// to a role. Empty list is a valid result (e.g. plain "user" has none).
func (r *permissionRepo) ListPermissionsForRole(ctx context.Context, role string) ([]string, error) {
	const q = `
		SELECT p.name
		FROM role_permissions rp
		JOIN permissions p ON p.id = rp.permission_id
		WHERE rp.role = $1
	`
	rows, err := r.db.Query(ctx, q, role)
	if err != nil {
		r.log.Error("permission repo: list for role", logger.Error(err))
		return nil, err
	}
	defer rows.Close()

	out := make([]string, 0, 16)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			continue
		}
		out = append(out, name)
	}
	return out, nil
}

func (r *permissionRepo) ListAllPermissions(ctx context.Context) ([]models.Permission, error) {
	const q = `SELECT id, name, COALESCE(description,''), created_at FROM permissions ORDER BY name`
	rows, err := r.db.Query(ctx, q)
	if err != nil {
		r.log.Error("permission repo: list all", logger.Error(err))
		return nil, err
	}
	defer rows.Close()

	var out []models.Permission
	for rows.Next() {
		var p models.Permission
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt); err != nil {
			continue
		}
		out = append(out, p)
	}
	return out, nil
}
