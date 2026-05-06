package service

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"convertpdfgo/api/models"
	"convertpdfgo/storage"
)

// fakePermStore counts DB calls so cache effectiveness is observable.
type fakePermStore struct {
	calls   int32
	byRole  map[string][]string
}

func (f *fakePermStore) ListPermissionsForRole(_ context.Context, role string) ([]string, error) {
	atomic.AddInt32(&f.calls, 1)
	return f.byRole[role], nil
}

func (f *fakePermStore) ListAllPermissions(_ context.Context) ([]models.Permission, error) {
	return nil, nil
}

// fakeStore plugs the fakePermStore into the wider storage interface so
// NewPermissionService can be constructed without a real DB. Only
// Permission() is called in this test; the rest can return nil safely.
type fakeStore struct {
	storage.IStorage
	perms storage.IPermissionStorage
}

func (s *fakeStore) Permission() storage.IPermissionStorage { return s.perms }

// fakeRedis: same minimal in-memory variant we use in other tests.
type fakeRedis struct {
	mu    sync.Mutex
	store map[string]string
}

func newFakeRedis() storage.IRedisStorage {
	return &fakeRedis{store: map[string]string{}}
}
func (f *fakeRedis) SetX(_ context.Context, k string, v interface{}, _ time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if s, ok := v.(string); ok {
		f.store[k] = s
	}
	return nil
}
func (f *fakeRedis) Get(_ context.Context, k string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.store[k], nil
}

// noopLogger to avoid pulling the zap dep into tests.
type noopLogger struct{}

func TestHasPermission_GrantsAndDenies(t *testing.T) {
	ps := &fakePermStore{byRole: map[string][]string{
		"admin":     {"users:read", "users:write"},
		"moderator": {"jobs:read"},
		"user":      {},
	}}
	svc := newTestPermissionService(ps)

	if !svc.HasPermission(context.Background(), "admin", "users:write") {
		t.Fatalf("admin must have users:write")
	}
	if svc.HasPermission(context.Background(), "moderator", "users:write") {
		t.Fatalf("moderator must NOT have users:write")
	}
	if svc.HasPermission(context.Background(), "user", "jobs:read") {
		t.Fatalf("plain user must have no perms")
	}
	if svc.HasPermission(context.Background(), "", "anything") {
		t.Fatalf("empty role must always deny")
	}
}

func TestPermissionsCacheReducesDBCalls(t *testing.T) {
	ps := &fakePermStore{byRole: map[string][]string{
		"admin": {"users:read", "users:write"},
	}}
	svc := newTestPermissionService(ps)

	// First lookup populates cache.
	if !svc.HasPermission(context.Background(), "admin", "users:read") {
		t.Fatalf("expected admin to have users:read")
	}
	// Subsequent lookups should hit the local cache, not the DB.
	for i := 0; i < 100; i++ {
		_ = svc.HasPermission(context.Background(), "admin", "users:read")
	}
	if got := atomic.LoadInt32(&ps.calls); got != 1 {
		t.Fatalf("cache miss count: want 1 DB call, got %d", got)
	}
}

func TestInvalidate_ForcesReload(t *testing.T) {
	ps := &fakePermStore{byRole: map[string][]string{
		"admin": {"users:read"},
	}}
	svc := newTestPermissionService(ps)

	_ = svc.HasPermission(context.Background(), "admin", "users:read")
	svc.Invalidate("admin")
	_ = svc.HasPermission(context.Background(), "admin", "users:read")

	if got := atomic.LoadInt32(&ps.calls); got != 2 {
		t.Fatalf("after invalidate: want 2 DB calls, got %d", got)
	}
}

// newTestPermissionService bypasses NewPermissionService (which expects an
// IStorage) and constructs the service directly. Keeps the test from
// having to fake every IStorage method.
func newTestPermissionService(ps storage.IPermissionStorage) PermissionService {
	return &permissionService{
		stg:        ps,
		redis:      newFakeRedis(),
		log:        nil, // unused on the cached paths under test
		localCache: make(map[string]localEntry),
	}
}
