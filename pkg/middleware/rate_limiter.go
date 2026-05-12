package middleware

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"

	"convertpdfgo/storage"
)

// rate.Every is a function call so it can't sit in a const block under
// Go's "constant expressions only" rule. Keep these as package-level vars
// (they're effectively immutable at runtime).
var (
	rateLimit       = rate.Every(200 * time.Millisecond) // 5 req/s sustained
	burst           = 10                                 // small headroom for UI bursts; abuse threshold is well above this
	visitorTTL      = 10 * time.Minute
	cleanupInterval = 5 * time.Minute

	// Abuse policy:
	//   * `abuseWindow` resets the violation counter so honest bursts
	//     don't accumulate over hours.
	//   * `abuseThreshold` is how many rejected requests in that window
	//     trip the 24-hour ban.
	//   * `banDuration` is the cool-off TTL on the ban key in Redis.
	abuseWindow    = 1 * time.Minute
	abuseThreshold = 10
	banDuration    = 24 * time.Hour
)

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

var (
	visitors = make(map[string]*visitor)
	mu       sync.Mutex
)

func init() {
	go cleanupVisitors()
}

func cleanupVisitors() {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		mu.Lock()
		cutoff := time.Now().Add(-visitorTTL)
		for key, v := range visitors {
			if v.lastSeen.Before(cutoff) {
				delete(visitors, key)
			}
		}
		mu.Unlock()
	}
}

func getVisitor(key string) *rate.Limiter {
	mu.Lock()
	defer mu.Unlock()

	v, exists := visitors[key]
	if !exists {
		v = &visitor{limiter: rate.NewLimiter(rateLimit, burst)}
		visitors[key] = v
	}
	v.lastSeen = time.Now()
	return v.limiter
}

// RateLimiterMiddleware enforces three things in order:
//
//  1. A Redis-backed 24h ban key. If present, the request short-
//     circuits without ever touching the in-memory limiter.
//  2. The in-memory token bucket (5 rps sustained, burst 10).
//  3. On rate-limit rejection, we increment a Redis counter that
//     survives across replicas; when it crosses abuseThreshold inside
//     abuseWindow, we set the ban key and reject for 24 h.
//
// Passing a nil redis turns off the Redis-backed pieces (ban check +
// abuse tracking) so unit tests don't need a live Redis. Production
// always passes a real client.
func RateLimiterMiddleware(redis storage.IRedisStorage) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := visitorKey(c)
		ctx := c.Request.Context()

		// (1) banned? — fast path with one GET.
		if redis != nil {
			if v, _ := redis.Get(ctx, banKey(key)); v != "" {
				c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
					"error":  "Temporarily blocked due to abuse",
					"retry":  "24h",
					"reason": "exceeded rate limit too many times",
				})
				return
			}
		}

		// (2) in-memory bucket.
		if getVisitor(key).Allow() {
			c.Next()
			return
		}

		// (3) reject, and count toward the ban threshold.
		if redis != nil {
			recordAbuse(ctx, redis, key)
		}
		c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "Too many requests"})
	}
}

// visitorKey picks the granularity the rate limiter (and the abuse /
// ban tables in Redis) bucket by. Authenticated callers share a
// single bucket across devices; guests share by remote IP only —
// earlier we appended User-Agent here, which meant UA-less callers
// all stomped on each other (WEAKNESSES.md #14).
func visitorKey(c *gin.Context) string {
	if userID, exists := c.Get("user_id"); exists {
		return "user:" + userID.(string)
	}
	return "ip:" + c.ClientIP()
}

// recordAbuse increments a counter under abuseKey(key) and, if it
// crosses abuseThreshold within abuseWindow, sets banKey(key) with
// banDuration TTL. The counter is intentionally lossy — Get → +1 →
// SetX — because our IRedisStorage doesn't expose INCR and exact
// off-by-one counting on abuse is fine (we already err generous).
func recordAbuse(ctx interface{}, redis storage.IRedisStorage, key string) {
	gctx := contextFromAny(ctx)
	abuseK := abuseKey(key)
	cur, _ := redis.Get(gctx, abuseK)
	n, _ := strconv.Atoi(cur)
	n++
	_ = redis.SetX(gctx, abuseK, strconv.Itoa(n), abuseWindow)
	if n >= abuseThreshold {
		_ = redis.SetX(gctx, banKey(key), "1", banDuration)
	}
}

func banKey(visitor string) string   { return "rate_banned:" + visitor }
func abuseKey(visitor string) string { return "rate_abuse:" + visitor }

// contextFromAny is a tiny shim so callers can pass either a Gin
// request context or a plain context.Context — keeps the call site
// in the middleware itself short.
func contextFromAny(v interface{}) ctxAny {
	if c, ok := v.(ctxAny); ok {
		return c
	}
	// gin's c.Request.Context() always satisfies context.Context;
	// this fallback is dead-code, kept defensive.
	return nil
}

// ctxAny aliases context.Context locally so storage.IRedisStorage's
// argument type is satisfied without forcing every middleware import
// of context.
type ctxAny = interface {
	Deadline() (time.Time, bool)
	Done() <-chan struct{}
	Err() error
	Value(any) any
}
