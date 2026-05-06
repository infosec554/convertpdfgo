package service

import (
	"context"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/storage"
)

type LogService interface {
	GetLogsByJobID(ctx context.Context, jobID string) ([]models.Log, error)
}

type logService struct {
	stg storage.IStorage
	log logger.ILogger
}

func NewLogService(stg storage.IStorage, log logger.ILogger) LogService {
	return &logService{stg: stg, log: log}
}

func (s *logService) GetLogsByJobID(ctx context.Context, jobID string) ([]models.Log, error) {
	s.log.Info("LogService.GetLogsByJobID called")

	logs, err := s.stg.Log().GetLogsByJobID(ctx, jobID)
	if err != nil {
		s.log.Error("failed to get logs", logger.Error(err))
		return nil, err
	}

	return logs, nil
}
