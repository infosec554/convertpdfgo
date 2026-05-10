package service

import (
	"context"
	"time"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/storage"
)

type AuditService interface {
	Record(ctx context.Context, e models.AuthAudit)
	ListByUser(ctx context.Context, userID string, limit int) ([]models.AuthAudit, error)
	ListSuspicious(ctx context.Context, withinMinutes, minFails int) ([]models.SuspiciousLogin, error)
}

type auditService struct {
	stg storage.IAuthAuditStorage
	log logger.ILogger
}

func NewAuditService(stg storage.IStorage, log logger.ILogger) AuditService {
	return &auditService{stg: stg.AuthAudit(), log: log}
}

// Record is best-effort: never blocks the caller's flow.
func (s *auditService) Record(ctx context.Context, e models.AuthAudit) {
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now()
	}
	go func(ev models.AuthAudit) {
		// Detached context with its own short timeout so a request-level
		// cancellation doesn't drop the audit row.
		bg, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = s.stg.Record(bg, ev)
	}(e)
}

func (s *auditService) ListByUser(ctx context.Context, userID string, limit int) ([]models.AuthAudit, error) {
	return s.stg.ListByUser(ctx, userID, limit)
}

func (s *auditService) ListSuspicious(ctx context.Context, withinMinutes, minFails int) ([]models.SuspiciousLogin, error) {
	return s.stg.ListSuspicious(ctx, withinMinutes, minFails)
}
