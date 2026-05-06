package service

import (
	"context"

	"convertpdfgo/pkg/logger"
	"convertpdfgo/storage"
)

type SysUserService interface {
	GetByPhone(ctx context.Context, phone string) (id, hashedPassword, status string, err error)
	Create(ctx context.Context, name, phone, hashedPassword, createdBy string) (string, error)
	AssignRoles(ctx context.Context, sysuserID string, roleIDs []string) error
}

type sysuserService struct {
	stg storage.ISysuserStorage
	log logger.ILogger
}

func NewSysUserService(stg storage.IStorage, log logger.ILogger) SysUserService {
	return &sysuserService{stg: stg.Sysuser(), log: log}
}

func (s *sysuserService) GetByPhone(ctx context.Context, phone string) (string, string, string, error) {
	id, hashedPassword, status, err := s.stg.GetByPhone(ctx, phone)
	if err != nil {
		s.log.Error("sysuser: get by phone", logger.Error(err))
	}
	return id, hashedPassword, status, err
}

func (s *sysuserService) Create(ctx context.Context, name, phone, hashedPassword, createdBy string) (string, error) {
	id, err := s.stg.Create(ctx, name, phone, hashedPassword, createdBy)
	if err != nil {
		s.log.Error("sysuser: create", logger.Error(err), logger.String("phone", phone))
		return "", err
	}
	s.log.Info("sysuser created", logger.String("sysuser_id", id))
	return id, nil
}

func (s *sysuserService) AssignRoles(ctx context.Context, sysuserID string, roleIDs []string) error {
	if err := s.stg.AssignRoles(ctx, sysuserID, roleIDs); err != nil {
		s.log.Error("sysuser: assign roles", logger.Error(err))
		return err
	}
	return nil
}
