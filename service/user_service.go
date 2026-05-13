package service

import (
	"context"
	"fmt"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/storage"
)

type UserService interface {
	Create(ctx context.Context, req models.CreateUser) (string, error)
	GetForLoginByEmail(ctx context.Context, email string) (models.LoginUser, error)
	GetByID(ctx context.Context, id string) (*models.User, error)
	ExistsByEmail(ctx context.Context, email string) (bool, error)
	UpdatePassword(ctx context.Context, id, hashedPassword string) error
	UpdateProfile(ctx context.Context, id, name string) error

	// Admin-only methods.
	UpdateRole(ctx context.Context, id, role string) error
	UpdateStatus(ctx context.Context, id, status string) error
	List(ctx context.Context, filter models.UserListFilter) ([]models.User, int, error)
}

type userService struct {
	stg storage.IUserStorage
	log logger.ILogger
}

func NewUserService(stg storage.IStorage, log logger.ILogger) UserService {
	return &userService{stg: stg.User(), log: log}
}

func (s *userService) Create(ctx context.Context, req models.CreateUser) (string, error) {
	id, err := s.stg.Create(ctx, req)
	if err != nil {
		s.log.Error("user: create", logger.Error(err), logger.String("email", req.Email))
		return "", err
	}
	s.log.Info("user created", logger.String("user_id", id), logger.String("email", req.Email))
	return id, nil
}

func (s *userService) GetForLoginByEmail(ctx context.Context, email string) (models.LoginUser, error) {
	return s.stg.GetForLoginByEmail(ctx, email)
}

func (s *userService) GetByID(ctx context.Context, id string) (*models.User, error) {
	return s.stg.GetByID(ctx, id)
}

func (s *userService) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	return s.stg.ExistsByEmail(ctx, email)
}

func (s *userService) UpdatePassword(ctx context.Context, id, hashedPassword string) error {
	if err := s.stg.UpdatePassword(ctx, id, hashedPassword); err != nil {
		s.log.Error("user: update password", logger.Error(err), logger.String("user_id", id))
		return err
	}
	s.log.Info("password updated", logger.String("user_id", id))
	return nil
}

func (s *userService) UpdateProfile(ctx context.Context, id, name string) error {
	if err := s.stg.UpdateProfile(ctx, id, name); err != nil {
		s.log.Error("user: update profile", logger.Error(err), logger.String("user_id", id))
		return err
	}
	s.log.Info("profile updated", logger.String("user_id", id))
	return nil
}

// validRoles are the only values UpdateRole will accept. Two-tier model
// only — moderator/superadmin collapsed into admin in migration 0014.
var validRoles = map[string]struct{}{
	"user":  {},
	"admin": {},
}

// validStatuses likewise mirrors the user_status_enum values used by the
// existing 0001 migration.
var validStatuses = map[string]struct{}{
	"active":  {},
	"blocked": {},
	"deleted": {},
}

func (s *userService) UpdateRole(ctx context.Context, id, role string) error {
	if _, ok := validRoles[role]; !ok {
		return fmt.Errorf("invalid role: %s", role)
	}
	if err := s.stg.UpdateRole(ctx, id, role); err != nil {
		s.log.Error("user: update role", logger.Error(err), logger.String("user_id", id))
		return err
	}
	s.log.Info("role updated", logger.String("user_id", id), logger.String("role", role))
	return nil
}

func (s *userService) UpdateStatus(ctx context.Context, id, status string) error {
	if _, ok := validStatuses[status]; !ok {
		return fmt.Errorf("invalid status: %s", status)
	}
	if err := s.stg.UpdateStatus(ctx, id, status); err != nil {
		s.log.Error("user: update status", logger.Error(err), logger.String("user_id", id))
		return err
	}
	s.log.Info("status updated", logger.String("user_id", id), logger.String("status", status))
	return nil
}

func (s *userService) List(ctx context.Context, filter models.UserListFilter) ([]models.User, int, error) {
	return s.stg.List(ctx, filter)
}
