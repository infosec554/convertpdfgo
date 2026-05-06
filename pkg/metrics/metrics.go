// Package metrics exposes Prometheus counters/histograms used to observe the
// async PDF pipeline. The /metrics endpoint is wired up by the API router.
package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// JobsEnqueued counts how many tasks each handler has put on the queue.
	JobsEnqueued = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "convertpdfgo",
			Subsystem: "queue",
			Name:      "jobs_enqueued_total",
			Help:      "Total number of jobs enqueued, partitioned by task type.",
		},
		[]string{"task"},
	)

	// JobsProcessed counts worker outcomes.
	JobsProcessed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "convertpdfgo",
			Subsystem: "worker",
			Name:      "jobs_processed_total",
			Help:      "Total number of jobs processed by the worker, partitioned by task and status.",
		},
		[]string{"task", "status"},
	)

	// JobDuration tracks how long each task type takes end-to-end.
	JobDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "convertpdfgo",
			Subsystem: "worker",
			Name:      "job_duration_seconds",
			Help:      "Histogram of job processing duration in seconds.",
			Buckets:   []float64{0.05, 0.1, 0.5, 1, 2, 5, 10, 30, 60, 120, 300},
		},
		[]string{"task"},
	)

	// CacheHits / CacheMisses cover the result-cache effectiveness.
	CacheHits = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "convertpdfgo",
			Subsystem: "cache",
			Name:      "hits_total",
			Help:      "Result-cache hits, partitioned by op.",
		},
		[]string{"op"},
	)
	CacheMisses = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "convertpdfgo",
			Subsystem: "cache",
			Name:      "misses_total",
			Help:      "Result-cache misses, partitioned by op.",
		},
		[]string{"op"},
	)

	// HTTP-side counters.
	HTTPRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "convertpdfgo",
			Subsystem: "http",
			Name:      "requests_total",
			Help:      "Total HTTP requests by method/path/status.",
		},
		[]string{"method", "path", "status"},
	)
	HTTPDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "convertpdfgo",
			Subsystem: "http",
			Name:      "request_duration_seconds",
			Help:      "Histogram of HTTP request latency.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)
)

func init() {
	prometheus.MustRegister(
		JobsEnqueued, JobsProcessed, JobDuration,
		CacheHits, CacheMisses,
		HTTPRequests, HTTPDuration,
	)
}

// Handler returns the /metrics handler that exposes all registered metrics
// in Prometheus exposition format.
func Handler() http.Handler {
	return promhttp.Handler()
}

// ObserveJob is a convenience wrapper for worker handlers.
//
//	defer metrics.ObserveJob("pdf:merge", time.Now())(err)
func ObserveJob(task string, start time.Time) func(error) {
	return func(err error) {
		JobDuration.WithLabelValues(task).Observe(time.Since(start).Seconds())
		status := "ok"
		if err != nil {
			status = "error"
		}
		JobsProcessed.WithLabelValues(task, status).Inc()
	}
}
