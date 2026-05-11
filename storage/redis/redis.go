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

// Keys runs SCAN under the hood — never use the blocking KEYS command on
// a production server. Returned at most maxKeys; capped at 10000 to avoid
// unbounded memory growth on a wildcard match.
func (r *redisRepo) Keys(ctx context.Context, pattern string, maxKeys int) ([]string, error) {
	if maxKeys <= 0 || maxKeys > 10000 {
		maxKeys = 1000
	}
	out := make([]string, 0, 64)
	iter := r.db.Scan(ctx, 0, pattern, 1000).Iterator()
	for iter.Next(ctx) {
		out = append(out, iter.Val())
		if len(out) >= maxKeys {
			break
		}
	}
	if err := iter.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// Del removes the key. Missing key is not an error.
func (r *redisRepo) Del(ctx context.Context, key string) error {
	return r.db.Del(ctx, key).Err()
}
