package main

import (
	"context"
	"errors"
	"net/http"
	_ "net/http/pprof" // /debug/pprof on the worker side too
	"time"

	"github.com/hibiken/asynq"

	"convertpdfgo/config"
	"convertpdfgo/pkg/gotenberg"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/pkg/mailer"
	"convertpdfgo/pkg/metrics"
	"convertpdfgo/pkg/queue"
	"convertpdfgo/service"
	"convertpdfgo/storage/postgres"
	"convertpdfgo/storage/redis"
)

func main() {
	cfg := config.Load()
	log := logger.NewWithLevel(cfg.ServiceName+"-worker", cfg.LoggerLevel)

	pgStore, err := postgres.New(context.Background(), cfg, log, nil)
	if err != nil {
		log.Error("worker: postgres connect failed", logger.Error(err))
		return
	}
	defer pgStore.Close()

	mailService := mailer.NewBrevo(cfg.BrevoAPIKey, cfg.BrevoSender)
	redisStore := redis.New(cfg)
	gotClient := gotenberg.New(cfg.GotenbergURL)

	services := service.New(pgStore, log, mailService, redisStore, gotClient, cfg)

	// Worker exposes its own /metrics on :8090 so Prometheus can scrape
	// JobDuration / JobsProcessed counters that only the worker writes to.
	// pprof rides on the same listener under /debug/pprof.
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", metrics.Handler())
		mux.Handle("/debug/pprof/", http.DefaultServeMux)
		mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		})
		srv := &http.Server{
			Addr:              ":8090",
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
		}
		log.Info("worker: metrics/pprof on :8090")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("worker: metrics server error", logger.Error(err))
		}
	}()

	srv := asynq.NewServer(
		queue.RedisOpt(cfg),
		asynq.Config{
			Concurrency: 20,
			Queues: map[string]int{
				queue.QueueCritical: 6,
				queue.QueueDefault:  3,
				queue.QueueLow:      1,
			},
		},
	)

	mux := asynq.NewServeMux()
	queue.NewHandlers(services, log).Register(mux)

	log.Info("worker: starting", logger.Int("concurrency", 20))
	if err := srv.Run(mux); err != nil {
		log.Error("worker: server stopped", logger.Error(err))
	}
}
