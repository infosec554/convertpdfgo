package cleaner

import (
	"context"
	"time"

	"convertpdfgo/pkg/logger"
	"convertpdfgo/service"
)

// StartFileCleanup schedules the long-horizon cleaner: every `interval`,
// remove files older than `retentionDays`. Use this for the safety net
// (default 6h / 7d).
func StartFileCleanup(services service.IServiceManager, log logger.ILogger, interval time.Duration, retentionDays int) (stop func()) {
	return startTicker(interval, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		count, err := services.File().CleanupOldFiles(ctx, retentionDays)
		if err != nil {
			log.Error("cleaner: cleanup failed", logger.Error(err))
			return
		}
		if count > 0 {
			log.Info("cleaner: removed old files", logger.Int("count", count), logger.Int("retention_days", retentionDays))
		}
	})
}

// StartGuestCleanup is the aggressive short-horizon cleaner. Every `interval`
// (e.g. 5 min) it deletes anonymous (guest) uploads older than `minutes`
// (e.g. 15 min). The goal: keep DB + disk clean of one-shot conversions
// without waiting for the daily safety net.
func StartGuestCleanup(services service.IServiceManager, log logger.ILogger, interval time.Duration, minutes int) (stop func()) {
	return startTicker(interval, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		count, err := services.File().CleanupGuestFiles(ctx, minutes)
		if err != nil {
			log.Error("cleaner: guest cleanup failed", logger.Error(err))
			return
		}
		if count > 0 {
			log.Info("cleaner: removed guest files", logger.Int("count", count), logger.Int("minutes", minutes))
		}
	})
}

// StartUniversalCleanup wipes BOTH guest and registered files older than
// `minutes`. Migration 0008 made *_jobs FKs CASCADE, so deleting from
// `files` also removes referencing job rows. Used as the privacy-first
// 5-minute cleaner — every conversion's input + output disappears soon
// after the user has had time to download.
func StartUniversalCleanup(services service.IServiceManager, log logger.ILogger, interval time.Duration, minutes int) (stop func()) {
	return startTicker(interval, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		count, err := services.File().CleanupAllFiles(ctx, minutes)
		if err != nil {
			log.Error("cleaner: universal cleanup failed", logger.Error(err))
			return
		}
		if count > 0 {
			log.Info("cleaner: removed files (universal)", logger.Int("count", count), logger.Int("minutes", minutes))
		}
	})
}

// startTicker runs fn now and on every tick. Returns a function that stops
// the ticker — call it on shutdown via defer.
func startTicker(interval time.Duration, fn func()) (stop func()) {
	ctx, cancel := context.WithCancel(context.Background())
	ticker := time.NewTicker(interval)

	go func() {
		fn()
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				fn()
			}
		}
	}()

	return cancel
}
