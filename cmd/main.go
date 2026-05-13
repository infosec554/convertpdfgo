package main

import (
	"context"
	"errors"
	"net/http"
	_ "net/http/pprof" // exposes /debug/pprof on the pprof port
	"os"
	"os/signal"
	"syscall"
	"time"

	"convertpdfgo/api"
	"convertpdfgo/config"
	"convertpdfgo/pkg/cleaner"
	"convertpdfgo/pkg/gotenberg"
	appjwt "convertpdfgo/pkg/jwt"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/pkg/mailer"
	"convertpdfgo/pkg/queue"
	"convertpdfgo/service"
	"convertpdfgo/storage/postgres"
	"convertpdfgo/storage/redis"
)

func main() {
	cfg := config.Load()
	log := logger.NewWithLevel(cfg.ServiceName, cfg.LoggerLevel)

	// Refuse to start with a placeholder / weak JWT secret in any
	// environment that opts in via JWT_REQUIRED=true (set in production).
	if os.Getenv("JWT_REQUIRED") == "true" {
		appjwt.MustValidateSecret()
	}

	pgStore, err := postgres.New(context.Background(), cfg, log, nil)
	if err != nil {
		log.Error("error while connecting to db", logger.Error(err))
		return
	}
	defer pgStore.Close()

	mailService := mailer.NewBrevo(cfg.BrevoAPIKey, cfg.BrevoSender)
	redis := redis.New(cfg)
	gotClient := gotenberg.New(cfg.GotenbergURL)

	queueClient := queue.NewClient(cfg)
	defer queueClient.Close()

	services := service.New(pgStore, log, mailService, redis, gotClient, cfg)
	router := api.New(services, log, cfg, queueClient, redis)

	// Background cleanup — privacy-first model:
	//   - primary: every 1min, drop ANY file (guest + registered) older
	//              than 5min. Once the user has had time to download,
	//              their data leaves the server. Migration 0008 made
	//              *_jobs FKs CASCADE so referencing job rows go too.
	//   - safety net: every 6h, drop anything still around past 1 day
	//                 (catches files that survived the primary loop due
	//                 to a transient DB error).
	stopCleaner := cleaner.StartFileCleanup(services, log, 6*time.Hour, 1)
	defer stopCleaner()
	stopUniversal := cleaner.StartUniversalCleanup(services, log, time.Minute, 5)
	defer stopUniversal()

	srv := &http.Server{
		Addr:    ":8080",
		Handler: router,
		// ReadHeaderTimeout: just the headers — short. Slowloris guard.
		ReadHeaderTimeout: 10 * time.Second,
		// ReadTimeout: whole request body. Long enough to let a slow mobile
		// connection push a 30 MB upload at 100 KB/s without timing out.
		ReadTimeout: 10 * time.Minute,
		// WriteTimeout: whole response. A linearised PDF + slow client must
		// finish within this window.
		WriteTimeout: 10 * time.Minute,
		IdleTimeout:  120 * time.Second,
		// 1 MiB header cap — Gin defaults to 1 MiB too, but pin it
		// explicitly so a Server tweak elsewhere doesn't lower it.
		MaxHeaderBytes: 1 << 20,
	}

	go func() {
		log.Info("Service is running on", logger.Int("port", 8080))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server error", logger.Error(err))
		}
	}()

	// pprof — bind to localhost only so it's not exposed publicly.
	go func() {
		pprofSrv := &http.Server{Addr: "127.0.0.1:6060", ReadHeaderTimeout: 5 * time.Second}
		if err := pprofSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("pprof server error", logger.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info("shutdown signal received, draining connections...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error("graceful shutdown failed", logger.Error(err))
	}
	log.Info("server stopped cleanly")
}
