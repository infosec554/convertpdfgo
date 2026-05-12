package service

import (
	"context"
	"sync"
	"time"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/storage"
)

// AdminStatsService — adminka uchun aggregat ma'lumotlar (statistika).
type AdminStatsService interface {
	UsersByDay(ctx context.Context, days int) ([]models.DailyCount, error)
	TopUsersByDisk(ctx context.Context, limit int) ([]models.TopUserDisk, error)
	UserBuckets(ctx context.Context) (map[string]int, error)
	// PublicCounts is the unauthenticated landing-page payload —
	// see GET /api/stats/public. Cached for publicCountsTTL so
	// /api/stats/public pollers can't amplify load on the DB.
	PublicCounts(ctx context.Context) (models.PublicCounts, error)
}

type adminStatsService struct {
	stg storage.IAdminStatsStorage
	log logger.ILogger

	// In-process cache for PublicCounts — a single COUNT-and-UNION
	// query is cheap but not free, and the landing page polls.
	publicMu     sync.Mutex
	publicCached models.PublicCounts
	publicSeen   time.Time
}

const publicCountsTTL = 30 * time.Second

func NewAdminStatsService(stg storage.IStorage, log logger.ILogger) AdminStatsService {
	return &adminStatsService{stg: stg.AdminStats(), log: log}
}

func (s *adminStatsService) UsersByDay(ctx context.Context, days int) ([]models.DailyCount, error) {
	return s.stg.UsersByDay(ctx, days)
}

func (s *adminStatsService) TopUsersByDisk(ctx context.Context, limit int) ([]models.TopUserDisk, error) {
	return s.stg.TopUsersByDisk(ctx, limit)
}

func (s *adminStatsService) UserBuckets(ctx context.Context) (map[string]int, error) {
	return s.stg.UserBuckets(ctx)
}

// PublicCounts returns the cached aggregate when fresh; otherwise
// re-queries and refreshes. A single mutex guards both the read and
// the refresh — concurrent first-readers serialise, which is fine
// for a once-per-30s call.
func (s *adminStatsService) PublicCounts(ctx context.Context) (models.PublicCounts, error) {
	s.publicMu.Lock()
	defer s.publicMu.Unlock()

	if time.Since(s.publicSeen) < publicCountsTTL && !s.publicSeen.IsZero() {
		return s.publicCached, nil
	}
	fresh, err := s.stg.PublicCounts(ctx)
	if err != nil {
		// Serve stale on transient DB error — better a slightly old
		// number than a 500 on the landing page.
		if !s.publicSeen.IsZero() {
			s.log.Warning("public stats: serving stale due to error", logger.Error(err))
			return s.publicCached, nil
		}
		return models.PublicCounts{}, err
	}
	s.publicCached = fresh
	s.publicSeen = time.Now()
	return fresh, nil
}
