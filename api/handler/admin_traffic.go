package handler

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"convertpdfgo/pkg/accesslog"
	"convertpdfgo/pkg/timeouts"
)

// ListTrafficAdmin godoc
// @Router   /admin/traffic [get]
// @Summary  Recent request log (IP, action, account/guest)
// @Tags     admin-traffic
// @Security ApiKeyAuth
func (h Handler) ListTrafficAdmin(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	items, err := accesslog.Recent(ctx, limit)
	if err != nil {
		handleResponse(c, h.log, "failed to list traffic", http.StatusInternalServerError, err.Error())
		return
	}
	handleResponse(c, h.log, "traffic", http.StatusOK, gin.H{"items": items})
}

// TrafficSummaryAdmin godoc
// @Router   /admin/traffic/summary [get]
// @Summary  Traffic aggregates (totals, unique IPs, top countries + paths)
// @Tags     admin-traffic
// @Security ApiKeyAuth
func (h Handler) TrafficSummaryAdmin(c *gin.Context) {
	hours, _ := strconv.Atoi(c.DefaultQuery("hours", "24"))

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	total, uniqueIPs, countries, paths, err := accesslog.Summary(ctx, hours)
	if err != nil {
		handleResponse(c, h.log, "failed to summarize traffic", http.StatusInternalServerError, err.Error())
		return
	}
	handleResponse(c, h.log, "traffic summary", http.StatusOK, gin.H{
		"total":      total,
		"unique_ips": uniqueIPs,
		"countries":  countries,
		"paths":      paths,
		"window_h":   hours,
	})
}
