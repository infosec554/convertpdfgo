package service

import (
	"context"
	"time"

	"convertpdfgo/pkg/logger"
	"convertpdfgo/storage"
)

type RedisService interface {
	SetX(ctx context.Context, key string, value interface{}, duration time.Duration) error
	Get(ctx context.Context, key string) (string, error)
}

type redisService struct {
	redis storage.IRedisStorage
	log   logger.ILogger
}

func NewRedisService(_ storage.IStorage, log logger.ILogger, redis storage.IRedisStorage) RedisService {
	return &redisService{redis: redis, log: log}
}

func (s *redisService) SetX(ctx context.Context, key string, value interface{}, duration time.Duration) error {
	if err := s.redis.SetX(ctx, key, value, duration); err != nil {
		s.log.Error("redis: set", logger.Error(err), logger.String("key", key))
		return err
	}
	return nil
}

func (s *redisService) Get(ctx context.Context, key string) (string, error) {
	val, err := s.redis.Get(ctx, key)
	if err != nil {
		return "", err
	}
	return val, nil
}
