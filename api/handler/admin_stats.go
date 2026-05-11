package handler

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/pkg/queue"
	"convertpdfgo/pkg/timeouts"
)

func modelsUserListFilter(role, status string, limit, offset int) models.UserListFilter {
	return models.UserListFilter{Role: role, Status: status, Limit: limit, Offset: offset}
}

// StatsOverviewAdmin godoc
// @Router       /admin/stats/overview [get]
// @Security     ApiKeyAuth
// @Summary      Overview counters (admin)
// @Tags         admin-stats
// @Success      200 {object} models.Response
func (h Handler) StatsOverviewAdmin(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	out := gin.H{
		"users":  h.statUserCounts(ctx),
		"queues": h.statQueueDepth(),
	}
	handleResponse(c, h.log, "overview", http.StatusOK, out)
}

// statUserCounts returns the dashboard's user-count summary in a single
// SQL round-trip via FILTER aggregates. Replaces the previous
// implementation that issued five User().List() calls — each of which
// did its own SELECT + COUNT(*), so the admin overview was 10 queries
// per refresh.
func (h Handler) statUserCounts(ctx context.Context) gin.H {
	buckets, err := h.services.AdminStats().UserBuckets(ctx)
	if err != nil {
		h.log.Error("admin_stats: user buckets", logger.Error(err))
		return gin.H{"error": err.Error()}
	}
	out := gin.H{}
	for k, v := range buckets {
		out[k] = v
	}
	return out
}

// statQueueDepth asks Asynq for pending/active/retry counts so the admin
// can spot a worker stall without opening Asynqmon.
func (h Handler) statQueueDepth() gin.H {
	inspector := asynq.NewInspector(queue.RedisOpt(h.cfg))
	defer inspector.Close()

	out := gin.H{}
	for _, q := range []string{queue.QueueCritical, queue.QueueDefault, queue.QueueLow} {
		info, err := inspector.GetQueueInfo(q)
		if err != nil {
			out[q] = gin.H{"error": err.Error()}
			continue
		}
		out[q] = gin.H{
			"pending":   info.Pending,
			"active":    info.Active,
			"retry":     info.Retry,
			"archived":  info.Archived,
			"completed": info.Completed,
			"failed":    info.Failed,
		}
	}
	return out
}

// StatsUsersByDayAdmin godoc
// @Router       /admin/stats/users [get]
// @Security     ApiKeyAuth
// @Summary      New users per day, last N days (admin)
// @Tags         admin-stats
// @Param        days query int false "lookback in days (default 30, max 365)"
// @Success      200 {object} models.Response
func (h Handler) StatsUsersByDayAdmin(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	if days <= 0 || days > 365 {
		days = 30
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	items, err := h.services.AdminStats().UsersByDay(ctx, days)
	if err != nil {
		handleResponse(c, h.log, "failed to fetch users-by-day", http.StatusInternalServerError, err.Error())
		return
	}
	handleResponse(c, h.log, "users by day", http.StatusOK, gin.H{
		"items": items,
		"days":  days,
	})
}

// StatsJobsAdmin godoc
// @Router       /admin/stats/jobs [get]
// @Security     ApiKeyAuth
// @Summary      Per-queue job counts (admin)
// @Description  Aggregates Asynq queue depth across all three priorities so
//
//	the dashboard can render a single bar chart of pending /
//	active / retry / archived / completed / failed totals.
//
// @Tags         admin-stats
// @Success      200 {object} models.Response
func (h Handler) StatsJobsAdmin(c *gin.Context) {
	inspector := asynq.NewInspector(queue.RedisOpt(h.cfg))
	defer inspector.Close()

	type queueStat struct {
		Queue     string `json:"queue"`
		Pending   int    `json:"pending"`
		Active    int    `json:"active"`
		Retry     int    `json:"retry"`
		Archived  int    `json:"archived"`
		Completed int    `json:"completed"`
		Failed    int    `json:"failed"`
	}

	out := make([]queueStat, 0, 3)
	totals := queueStat{Queue: "_total"}
	for _, q := range []string{queue.QueueCritical, queue.QueueDefault, queue.QueueLow} {
		info, err := inspector.GetQueueInfo(q)
		if err != nil {
			h.log.Error("admin_stats: queue info", logger.Error(err), logger.String("queue", q))
			continue
		}
		s := queueStat{
			Queue:     q,
			Pending:   info.Pending,
			Active:    info.Active,
			Retry:     info.Retry,
			Archived:  info.Archived,
			Completed: info.Completed,
			Failed:    info.Failed,
		}
		out = append(out, s)
		totals.Pending += s.Pending
		totals.Active += s.Active
		totals.Retry += s.Retry
		totals.Archived += s.Archived
		totals.Completed += s.Completed
		totals.Failed += s.Failed
	}

	handleResponse(c, h.log, "job stats", http.StatusOK, gin.H{
		"queues": out,
		"total":  totals,
	})
}

// StatsStorageAdmin godoc
// @Router       /admin/stats/storage [get]
// @Security     ApiKeyAuth
// @Summary      Top disk consumers (admin)
// @Tags         admin-stats
// @Param        limit query int false "default 50, max 200"
// @Success      200 {object} models.Response
func (h Handler) StatsStorageAdmin(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	items, err := h.services.AdminStats().TopUsersByDisk(ctx, limit)
	if err != nil {
		handleResponse(c, h.log, "failed to fetch storage stats", http.StatusInternalServerError, err.Error())
		return
	}
	handleResponse(c, h.log, "top disk users", http.StatusOK, gin.H{
		"items": items,
		"limit": limit,
	})
}

// StatsCacheAdmin godoc
// @Router       /admin/stats/cache [get]
// @Security     ApiKeyAuth
// @Summary      Cache hit-rate pointer (admin)
// @Description  Cache counters (hits/misses) live in Prometheus, scraped
//
//	from /metrics. This endpoint just points the operator at
//	the right query so the dashboard URL doesn't have to be
//	hardcoded across multiple frontends.
//
// @Tags         admin-stats
// @Success      200 {object} models.Response
func (h Handler) StatsCacheAdmin(c *gin.Context) {
	handleResponse(c, h.log, "cache metrics live in Prometheus", http.StatusOK, gin.H{
		"prometheus_query":     `sum by (op) (rate(convertpdfgo_cache_hits_total[5m])) / (sum by (op) (rate(convertpdfgo_cache_hits_total[5m])) + sum by (op) (rate(convertpdfgo_cache_misses_total[5m])))`,
		"prometheus_url_hint":  "http://localhost:9090",
		"grafana_url_hint":     "http://localhost:3001",
		"raw_metrics_endpoint": "/metrics",
		"counters": []string{
			"convertpdfgo_cache_hits_total",
			"convertpdfgo_cache_misses_total",
		},
	})
}
