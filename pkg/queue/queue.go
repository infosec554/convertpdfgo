package queue

import (
	"fmt"

	"github.com/hibiken/asynq"

	"convertpdfgo/config"
)

const (
	QueueCritical = "critical"
	QueueDefault  = "default"
	QueueLow      = "low"
)

// RedisOpt builds asynq.RedisClientOpt from app config.
func RedisOpt(cfg config.Config) asynq.RedisClientOpt {
	return asynq.RedisClientOpt{
		Addr:     fmt.Sprintf("%s:%s", cfg.RedisHost, cfg.RedisPort),
		Password: cfg.RedisPassword,
		DB:       0,
	}
}

// NewClient creates an Asynq client used by API handlers to enqueue jobs.
func NewClient(cfg config.Config) *asynq.Client {
	return asynq.NewClient(RedisOpt(cfg))
}
