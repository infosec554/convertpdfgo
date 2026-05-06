package service

import (
	"context"
	"strings"
	"sync"
	"time"

	"convertpdfgo/pkg/logger"
	"convertpdfgo/storage"
)

// PermissionService answers "does role X have permission Y?" with two
// layers of caching: an in-process map (fast, per-replica) backed by a
// Redis SET (so a fresh replica or a stale local cache still gets data
// without hitting Postgres on every request).
type PermissionService interface {
	HasPermission(ctx context.Context, role, perm string) bool
	PermissionsForRole(ctx context.Context, role string) ([]string, error)
	Invalidate(role string)
}

type permissionService struct {
	stg   storage.IPermissionStorage
	redis storage.IRedisStorage
	log   logger.ILogger

	mu         sync.RWMutex
	localCache map[string]localEntry
}

type localEntry struct {
	perms     map[string]struct{}
	expiresAt time.Time
}

const (
	localTTL = 1 * time.Minute
	redisTTL = 5 * time.Minute
)

func NewPermissionService(stg storage.IStorage, redis storage.IRedisStorage, log logger.ILogger) PermissionService {
	return &permissionService{
		stg:        stg.Permission(),
		redis:      redis,
		log:        log,
		localCache: make(map[string]localEntry),
	}
}

// HasPermission is the hot path — it must stay allocation-free in the
// common case (cached role lookup).
func (s *permissionService) HasPermission(ctx context.Context, role, perm string) bool {
	if role == "" || perm == "" {
		return false
	}
	perms, err := s.PermissionsForRole(ctx, role)
	if err != nil {
		// Fail closed: a DB hiccup must not silently grant access.
		return false
	}
	for _, p := range perms {
		if p == perm {
			return true
		}
	}
	return false
}

// PermissionsForRole returns every wire-format permission a role has.
func (s *permissionService) PermissionsForRole(ctx context.Context, role string) ([]string, error) {
	// Layer 1: in-process cache.
	if perms, ok := s.lookupLocal(role); ok {
		return perms, nil
	}

	// Layer 2: Redis. Avoids a DB round-trip when a sibling replica
	// already populated the entry.
	if s.redis != nil {
		if v, err := s.redis.Get(ctx, redisKeyForRole(role)); err == nil && v != "" {
			perms := strings.Split(v, ",")
			s.storeLocal(role, perms)
			return perms, nil
		}
	}

	// Layer 3: Postgres.
	perms, err := s.stg.ListPermissionsForRole(ctx, role)
	if err != nil {
		s.log.Error("permission: db lookup", logger.Error(err), logger.String("role", role))
		return nil, err
	}
	s.storeLocal(role, perms)
	if s.redis != nil {
		_ = s.redis.SetX(ctx, redisKeyForRole(role), strings.Join(perms, ","), redisTTL)
	}
	return perms, nil
}

// Invalidate drops the local cache entry for a role. Call after granting
// or revoking permissions so the change becomes visible without waiting
// for TTL expiry. (Redis entry will refresh from DB on next miss.)
func (s *permissionService) Invalidate(role string) {
	s.mu.Lock()
	delete(s.localCache, role)
	s.mu.Unlock()
}

func (s *permissionService) lookupLocal(role string) ([]string, bool) {
	s.mu.RLock()
	entry, ok := s.localCache[role]
	s.mu.RUnlock()
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	out := make([]string, 0, len(entry.perms))
	for p := range entry.perms {
		out = append(out, p)
	}
	return out, true
}

func (s *permissionService) storeLocal(role string, perms []string) {
	set := make(map[string]struct{}, len(perms))
	for _, p := range perms {
		if p != "" {
			set[p] = struct{}{}
		}
	}
	s.mu.Lock()
	s.localCache[role] = localEntry{perms: set, expiresAt: time.Now().Add(localTTL)}
	s.mu.Unlock()
}

func redisKeyForRole(role string) string { return "rbac:role:" + role }
