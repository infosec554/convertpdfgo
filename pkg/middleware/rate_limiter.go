package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// rate.Every is a function call so it can't sit in a const block under
// Go's "constant expressions only" rule. Keep these as package-level vars
// (they're effectively immutable at runtime).
var (
	rateLimit       = rate.Every(200 * time.Millisecond) // 5 req/s sustained
	burst           = 30                                 // soft start; absorbs UI bursts and smoke tests
	visitorTTL      = 10 * time.Minute
	cleanupInterval = 5 * time.Minute
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

func RateLimiterMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Identify the caller. Earlier the IP path appended User-Agent
		// to the key, which meant any client that sent an empty UA
		// (curl in some configs, healthchecks, scrapers) ended up
		// sharing a single bucket with every other UA-less caller —
		// pulling each other into the rate limit. WEAKNESSES.md #14.
		// IP alone is the right key for anonymous traffic.
		var key string
		if userID, exists := c.Get("user_id"); exists {
			key = "user:" + userID.(string)
		} else {
			key = "ip:" + c.ClientIP()
		}

		if !getVisitor(key).Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "Too many requests"})
			return
		}

		c.Next()
	}
}
