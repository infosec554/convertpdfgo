package loginlimiter

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

	"convertpdfgo/storage"
)

// fakeRedis implements storage.IRedisStorage in memory so tests don't
// need a real Redis. Only the two methods the limiter uses are real;
// expiry is approximated with a wall-clock deadline.
type fakeRedis struct {
	mu      sync.Mutex
	store   map[string]string
	expires map[string]time.Time
}

func newFakeRedis() storage.IRedisStorage {
	return &fakeRedis{
		store:   make(map[string]string),
		expires: make(map[string]time.Time),
	}
}

func (f *fakeRedis) SetX(_ context.Context, key string, value interface{}, ttl time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.store[key] = stringify(value)
	f.expires[key] = time.Now().Add(ttl)
	return nil
}

func (f *fakeRedis) Get(_ context.Context, key string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if exp, ok := f.expires[key]; ok && time.Now().After(exp) {
		delete(f.store, key)
		delete(f.expires, key)
		return "", nil
	}
	v, ok := f.store[key]
	if !ok {
		return "", nil
	}
	return v, nil
}

func (f *fakeRedis) Keys(_ context.Context, _ string, _ int) ([]string, error) {
	return nil, nil
}

func (f *fakeRedis) Del(_ context.Context, _ string) error {
	return nil
}

func stringify(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case int:
		return strconv.Itoa(t)
	default:
		return ""
	}
}

func TestLimiter_LocksAfterMaxFails(t *testing.T) {
	rs := newFakeRedis()
	lim := New(rs, 3, time.Minute)
	ctx := context.Background()

	// First two failures: not locked.
	for i := 0; i < 2; i++ {
		if err := lim.Check(ctx, "alice@example.com"); err != nil {
			t.Fatalf("attempt %d: should not be locked yet, got %v", i, err)
		}
		lim.RecordFailure(ctx, "alice@example.com")
	}

	// Third failure: counter hits maxFails. Next Check blocks.
	lim.RecordFailure(ctx, "alice@example.com")
	if err := lim.Check(ctx, "alice@example.com"); err != ErrLocked {
		t.Fatalf("after 3 fails: want ErrLocked, got %v", err)
	}
}

func TestLimiter_ResetClears(t *testing.T) {
	rs := newFakeRedis()
	lim := New(rs, 2, time.Minute)
	ctx := context.Background()

	lim.RecordFailure(ctx, "bob@example.com")
	lim.RecordFailure(ctx, "bob@example.com")
	if err := lim.Check(ctx, "bob@example.com"); err != ErrLocked {
		t.Fatalf("setup: want ErrLocked, got %v", err)
	}

	lim.Reset(ctx, "bob@example.com")
	// Reset writes "0" with 1s TTL — sleep past TTL so the entry is
	// genuinely gone, mimicking what the real implementation aims for.
	time.Sleep(1100 * time.Millisecond)
	if err := lim.Check(ctx, "bob@example.com"); err != nil {
		t.Fatalf("after reset: want nil, got %v", err)
	}
}

func TestLimiter_NilRedisIsNoOp(t *testing.T) {
	lim := New(nil, 5, time.Minute)
	if err := lim.Check(context.Background(), "x@example.com"); err != nil {
		t.Fatalf("nil redis must fail-open, got %v", err)
	}
	// Should not panic.
	lim.RecordFailure(context.Background(), "x@example.com")
	lim.Reset(context.Background(), "x@example.com")
}

func TestLimiter_PerEmailIsolation(t *testing.T) {
	rs := newFakeRedis()
	lim := New(rs, 2, time.Minute)
	ctx := context.Background()

	// Alice fails twice → locked.
	lim.RecordFailure(ctx, "alice@example.com")
	lim.RecordFailure(ctx, "alice@example.com")
	if err := lim.Check(ctx, "alice@example.com"); err != ErrLocked {
		t.Fatalf("alice: want ErrLocked, got %v", err)
	}

	// Bob's counter is independent.
	if err := lim.Check(ctx, "bob@example.com"); err != nil {
		t.Fatalf("bob: want nil, got %v", err)
	}
}
