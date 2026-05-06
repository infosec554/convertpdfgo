package blacklist

import (
	"context"
	"sync"
	"testing"
	"time"

	"convertpdfgo/storage"
)

// fakeRedis: minimal in-memory IRedisStorage. Cache and loginlimiter have
// near-identical helpers; we keep this one local rather than create a
// shared test helper package, since the surface area is tiny.
type fakeRedis struct {
	mu    sync.Mutex
	store map[string]string
	exp   map[string]time.Time
}

func newFakeRedis() storage.IRedisStorage {
	return &fakeRedis{store: map[string]string{}, exp: map[string]time.Time{}}
}

func (f *fakeRedis) SetX(_ context.Context, k string, v interface{}, ttl time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if s, ok := v.(string); ok {
		f.store[k] = s
	}
	f.exp[k] = time.Now().Add(ttl)
	return nil
}

func (f *fakeRedis) Get(_ context.Context, k string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if e, ok := f.exp[k]; ok && time.Now().After(e) {
		delete(f.store, k)
		delete(f.exp, k)
		return "", nil
	}
	v, ok := f.store[k]
	if !ok {
		return "", nil
	}
	return v, nil
}

func TestRevokeAndCheck(t *testing.T) {
	bl := New(newFakeRedis())
	ctx := context.Background()

	// Not revoked initially.
	if bl.IsRevoked(ctx, "jti-1") {
		t.Fatalf("fresh JTI must not be revoked")
	}

	// Revoke with future expiry.
	if err := bl.Revoke(ctx, "jti-1", time.Now().Add(time.Minute)); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if !bl.IsRevoked(ctx, "jti-1") {
		t.Fatalf("after revoke: must be revoked")
	}

	// Different JTI is unaffected.
	if bl.IsRevoked(ctx, "jti-2") {
		t.Fatalf("unrelated JTI must not be revoked")
	}
}

func TestRevoke_PastExpiryIsNoop(t *testing.T) {
	bl := New(newFakeRedis())
	ctx := context.Background()
	// Past expiry → Revoke should silently succeed without storing.
	if err := bl.Revoke(ctx, "jti-x", time.Now().Add(-time.Hour)); err != nil {
		t.Fatalf("revoke past: %v", err)
	}
	if bl.IsRevoked(ctx, "jti-x") {
		t.Fatalf("past-expiry revoke must not poison the blacklist")
	}
}

func TestRevoke_TTLClears(t *testing.T) {
	bl := New(newFakeRedis())
	ctx := context.Background()
	// Revoke for a very short window.
	if err := bl.Revoke(ctx, "jti-tt", time.Now().Add(50*time.Millisecond)); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if !bl.IsRevoked(ctx, "jti-tt") {
		t.Fatalf("just after revoke: must be revoked")
	}
	time.Sleep(120 * time.Millisecond)
	if bl.IsRevoked(ctx, "jti-tt") {
		t.Fatalf("after TTL: must not be revoked")
	}
}

func TestRevoke_NilBacking_Safe(t *testing.T) {
	var bl *Blacklist
	ctx := context.Background()
	// Both methods must accept a nil receiver without panicking.
	if err := bl.Revoke(ctx, "x", time.Now().Add(time.Minute)); err != nil {
		t.Fatalf("nil revoke: %v", err)
	}
	if bl.IsRevoked(ctx, "x") {
		t.Fatalf("nil receiver should fail-open (not revoked)")
	}
}

func TestRevoke_EmptyJTI(t *testing.T) {
	bl := New(newFakeRedis())
	ctx := context.Background()
	if err := bl.Revoke(ctx, "", time.Now().Add(time.Minute)); err != nil {
		t.Fatalf("empty jti: %v", err)
	}
	if bl.IsRevoked(ctx, "") {
		t.Fatalf("empty jti must never be revoked")
	}
}
