package handler

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"convertpdfgo/pkg/timeouts"
)

// ListAuditAdmin godoc
// @Router       /admin/audit [get]
// @Security     ApiKeyAuth
// @Summary      List auth audit events (admin)
// @Tags         admin-audit
// @Param        user_id query string false "filter by user id"
// @Param        limit   query int    false "default 100, max 500"
// @Success      200 {object} models.Response
func (h Handler) ListAuditAdmin(c *gin.Context) {
	userID := c.Query("user_id")
	limit, _ := strconv.Atoi(c.Query("limit"))
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	if userID == "" {
		// No user filter: return events for the requester only — admins
		// who want a global feed should pass user_id explicitly to keep
		// PII reads intentional.
		userID = c.GetString("user_id")
	}

	events, err := h.services.Audit().ListByUser(ctx, userID, limit)
	if err != nil {
		handleResponse(c, h.log, "failed to fetch audit", http.StatusInternalServerError, err.Error())
		return
	}
	handleResponse(c, h.log, "audit events", http.StatusOK, gin.H{
		"items":   events,
		"user_id": userID,
		"limit":   limit,
	})
}

// AuditByUserAdmin godoc
// @Router       /admin/audit/user/{id} [get]
// @Security     ApiKeyAuth
// @Summary      Audit events for a specific user (admin)
// @Tags         admin-audit
// @Param        id    path  string true  "user id"
// @Param        limit query int    false "default 100, max 500"
// @Success      200 {object} models.Response
func (h Handler) AuditByUserAdmin(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		handleResponse(c, h.log, "missing user id", http.StatusBadRequest, nil)
		return
	}

	limit, _ := strconv.Atoi(c.Query("limit"))
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	events, err := h.services.Audit().ListByUser(ctx, userID, limit)
	if err != nil {
		handleResponse(c, h.log, "failed to fetch audit", http.StatusInternalServerError, err.Error())
		return
	}
	handleResponse(c, h.log, "user audit", http.StatusOK, gin.H{
		"items":   events,
		"user_id": userID,
		"limit":   limit,
	})
}

// RecentActivityAdmin godoc
// @Router       /admin/audit/recent [get]
// @Security     ApiKeyAuth
// @Summary      Latest auth events across every user (admin)
// @Description  Powers the dashboard's "recent activity" feed. Sorted
//
//	newest-first; capped at 500 to keep responses fast.
//
// @Tags         admin-audit
// @Param        limit query int false "default 50, max 500"
// @Success      200 {object} models.Response
func (h Handler) RecentActivityAdmin(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limit <= 0 || limit > 500 {
		limit = 50
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	events, err := h.services.Audit().ListRecent(ctx, limit)
	if err != nil {
		handleResponse(c, h.log, "failed to fetch recent activity", http.StatusInternalServerError, err.Error())
		return
	}
	handleResponse(c, h.log, "recent activity", http.StatusOK, gin.H{
		"items": events,
		"limit": limit,
	})
}

// SuspiciousLoginsAdmin godoc
// @Router       /admin/audit/suspicious [get]
// @Security     ApiKeyAuth
// @Summary      Emails with abnormal login_fail rate (admin)
// @Description  Aggregates over the last `within` minutes (default 60).
//
//	Reports any email with >= `min_fails` failures (default 5),
//	grouping by email and listing distinct IPs.
//
// @Tags         admin-audit
// @Param        within    query int false "lookback window in minutes (default 60)"
// @Param        min_fails query int false "threshold (default 5)"
// @Success      200 {object} models.Response
func (h Handler) SuspiciousLoginsAdmin(c *gin.Context) {
	within, _ := strconv.Atoi(c.DefaultQuery("within", "60"))
	minFails, _ := strconv.Atoi(c.DefaultQuery("min_fails", "5"))

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	items, err := h.services.Audit().ListSuspicious(ctx, within, minFails)
	if err != nil {
		handleResponse(c, h.log, "failed to fetch suspicious", http.StatusInternalServerError, err.Error())
		return
	}
	handleResponse(c, h.log, "suspicious logins", http.StatusOK, gin.H{
		"items":          items,
		"within_minutes": within,
		"min_fails":      minFails,
	})
}
