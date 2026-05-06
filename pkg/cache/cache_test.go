package cache

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"convertpdfgo/storage"
)

// fakeRedis is a minimal in-memory IRedisStorage. We don't pull it from
// the loginlimiter test (different package, would need to export) — a
// few duplicated lines here are cheaper than that coupling.
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
	f.store[k] = toString(v)
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

func toString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func TestSetAndGet_HitsAndMisses(t *testing.T) {
	c := New(newFakeRedis())
	ctx := context.Background()

	// Miss.
	if _, err := c.Get(ctx, "compress", "missing"); !errors.Is(err, ErrMiss) {
		t.Fatalf("missing key: want ErrMiss, got %v", err)
	}

	// Set + hit.
	if err := c.Set(ctx, "k1", "v1", time.Minute); err != nil {
		t.Fatalf("set: %v", err)
	}
	got, err := c.Get(ctx, "compress", "k1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got != "v1" {
		t.Fatalf("get: want v1, got %s", got)
	}
}

func TestNilCache_Safe(t *testing.T) {
	var c *ResultCache
	ctx := context.Background()

	if _, err := c.Get(ctx, "compress", "any"); !errors.Is(err, ErrMiss) {
		t.Fatalf("nil cache Get: want ErrMiss, got %v", err)
	}
	if err := c.Set(ctx, "any", "v", time.Minute); err != nil {
		t.Fatalf("nil cache Set must noop, got %v", err)
	}
}

func TestKeys(t *testing.T) {
	if got := CompressKey("abc", "high"); !strings.HasPrefix(got, "pdf:compress:abc:") {
		t.Fatalf("compress key: %s", got)
	}
	if got := ConvertKey("word_to_pdf", "abc"); got != "pdf:word_to_pdf:abc" {
		t.Fatalf("convert key: %s", got)
	}
	if got := MergeKey("a,b,c"); got != "pdf:merge:a,b,c" {
		t.Fatalf("merge key: %s", got)
	}
}
