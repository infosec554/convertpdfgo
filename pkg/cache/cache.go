package cache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"convertpdfgo/pkg/metrics"
	"convertpdfgo/storage"
)

// ErrMiss is returned when a key has no entry.
var ErrMiss = errors.New("cache miss")

// ResultCache wraps the project's Redis client to memoise PDF operation
// outputs by content hash + parameter signature.
type ResultCache struct {
	rs storage.IRedisStorage
}

func New(rs storage.IRedisStorage) *ResultCache {
	return &ResultCache{rs: rs}
}

// Get returns the cached output file ID for the given key, or ErrMiss.
// Records hit/miss to Prometheus, partitioned by `op` (caller-supplied
// operation name like "compress", "word_to_pdf").
func (c *ResultCache) Get(ctx context.Context, op, key string) (string, error) {
	if c == nil || c.rs == nil {
		return "", ErrMiss
	}
	v, err := c.rs.Get(ctx, key)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			recordMiss(op)
			return "", ErrMiss
		}
		return "", err
	}
	if v == "" {
		recordMiss(op)
		return "", ErrMiss
	}
	recordHit(op)
	return v, nil
}

// Set stores an output file ID for the key with the given TTL.
// Failures are non-fatal — caching is best-effort.
func (c *ResultCache) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	if c == nil || c.rs == nil {
		return nil
	}
	return c.rs.SetX(ctx, key, value, ttl)
}

func recordHit(op string)  { metrics.CacheHits.WithLabelValues(op).Inc() }
func recordMiss(op string) { metrics.CacheMisses.WithLabelValues(op).Inc() }

// CompressKey builds the cache key for a compress operation.
func CompressKey(sha, level string) string {
	return fmt.Sprintf("pdf:compress:%s:%s", sha, level)
}

// ConvertKey builds the cache key for a Gotenberg conversion. opType is e.g.
// "word_to_pdf", "excel_to_pdf", etc.
func ConvertKey(opType, sha string) string {
	return fmt.Sprintf("pdf:%s:%s", opType, sha)
}

// MergeKey builds a cache key for a merge job from the joined sorted hashes
// of input files.
func MergeKey(joinedSHAs string) string {
	return "pdf:merge:" + joinedSHAs
}
