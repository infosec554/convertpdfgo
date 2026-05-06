package redis

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"

	"convertpdfgo/config"
	"convertpdfgo/storage"
)

type redisRepo struct {
	db *redis.Client
}

func New(cfg config.Config) storage.IRedisStorage {
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisHost + ":" + cfg.RedisPort,
		Password: cfg.RedisPassword,
		DB:       0, 
	})

	return &redisRepo{
		db: redisClient,
	}
}

func (r *redisRepo) SetX(ctx context.Context, key string, value interface{}, duration time.Duration) error {
	return r.db.SetEx(ctx, key, value, duration).Err()
}

func (r *redisRepo) Get(ctx context.Context, key string) (string, error) {
	result, err := r.db.Get(ctx, key).Result()
	if err != nil {
		return "", err
	}
	return result, nil
}
