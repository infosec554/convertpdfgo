package service

import (
	"context"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/storage"
)

type RoleService interface {
	Create(ctx context.Context, name, createdBy string) (string, error)
	Update(ctx context.Context, id, name string) error
	GetAll(ctx context.Context) ([]models.Role, error)
	Exists(ctx context.Context, id string) (bool, error)
}

type roleService struct {
	stg storage.IRoleStorage
	log logger.ILogger
}

func NewRoleService(stg storage.IStorage, log logger.ILogger) RoleService {
	return &roleService{stg: stg.Role(), log: log}
}

func (s *roleService) Create(ctx context.Context, name, createdBy string) (string, error) {
	id, err := s.stg.Create(ctx, name, createdBy)
	if err != nil {
		s.log.Error("role: create", logger.Error(err), logger.String("name", name))
		return "", err
	}
	s.log.Info("role created", logger.String("role_id", id), logger.String("name", name))
	return id, nil
}

func (s *roleService) Update(ctx context.Context, id, name string) error {
	if err := s.stg.Update(ctx, id, name); err != nil {
		s.log.Error("role: update", logger.Error(err), logger.String("id", id))
		return err
	}
	return nil
}

func (s *roleService) GetAll(ctx context.Context) ([]models.Role, error) {
	roles, err := s.stg.GetAll(ctx)
	if err != nil {
		s.log.Error("role: get all", logger.Error(err))
		return nil, err
	}
	return roles, nil
}

func (s *roleService) Exists(ctx context.Context, id string) (bool, error) {
	ok, err := s.stg.Exists(ctx, id)
	if err != nil {
		s.log.Error("role: exists", logger.Error(err), logger.String("id", id))
		return false, err
	}
	return ok, nil
}
