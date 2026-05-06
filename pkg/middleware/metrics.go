package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"convertpdfgo/pkg/metrics"
)

// PrometheusMetrics records request count + latency, partitioned by method
// and the *route template* (not the raw URL — so /file/abc-123 is bucketed
// as /file/:id, not as a unique path).
//
// Skips the /metrics endpoint itself so Prometheus doesn't measure its own
// scrapes.
func PrometheusMetrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.FullPath()
		if path == "" || path == "/metrics" {
			c.Next()
			return
		}

		start := time.Now()
		c.Next()

		dur := time.Since(start).Seconds()
		status := strconv.Itoa(c.Writer.Status())

		metrics.HTTPRequests.WithLabelValues(c.Request.Method, path, status).Inc()
		metrics.HTTPDuration.WithLabelValues(c.Request.Method, path).Observe(dur)
	}
}
