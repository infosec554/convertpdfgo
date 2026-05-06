package service

import (
	"context"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/storage"
)

type StatsService interface {
	GetUserStats(ctx context.Context, userID string) (models.UserStats, error)
}

type statsService struct {
	stg storage.IStorage
	log logger.ILogger
}

func NewStatsService(stg storage.IStorage, log logger.ILogger) StatsService {
	return &statsService{stg: stg, log: log}
}

func (s *statsService) GetUserStats(ctx context.Context, userID string) (models.UserStats, error) {
	s.log.Info("StatsService.GetUserStats called")

	stats, err := s.stg.Stat().GetUserStats(ctx, userID)
	if err != nil {
		s.log.Error("failed to get user stats", logger.Error(err))
		return models.UserStats{}, err
	}

	return stats, nil
}
