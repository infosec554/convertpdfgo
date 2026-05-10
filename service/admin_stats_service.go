package service

import (
	"context"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/storage"
)

// AdminStatsService — adminka uchun aggregat ma'lumotlar (statistika).
type AdminStatsService interface {
	UsersByDay(ctx context.Context, days int) ([]models.DailyCount, error)
	TopUsersByDisk(ctx context.Context, limit int) ([]models.TopUserDisk, error)
}

type adminStatsService struct {
	stg storage.IAdminStatsStorage
	log logger.ILogger
}

func NewAdminStatsService(stg storage.IStorage, log logger.ILogger) AdminStatsService {
	return &adminStatsService{stg: stg.AdminStats(), log: log}
}

func (s *adminStatsService) UsersByDay(ctx context.Context, days int) ([]models.DailyCount, error) {
	return s.stg.UsersByDay(ctx, days)
}

func (s *adminStatsService) TopUsersByDisk(ctx context.Context, limit int) ([]models.TopUserDisk, error) {
	return s.stg.TopUsersByDisk(ctx, limit)
}
