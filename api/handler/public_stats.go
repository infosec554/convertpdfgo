package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"convertpdfgo/pkg/timeouts"
)

// GetPublicStats — un-authenticated counter snapshot for the landing
// page. Service-side cached at 30 s so this can sit outside auth
// without inviting a DB-load DoS.
//
// @Summary  Public usage counters (total users, files uploaded, files processed)
// @Tags     stats
// @Produce  json
// @Success  200 {object} models.PublicCounts
// @Router   /api/stats/public [get]
func (h Handler) GetPublicStats(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	stats, err := h.services.AdminStats().PublicCounts(ctx)
	if err != nil {
		handleResponse(c, h.log, "public stats failed", http.StatusInternalServerError, err.Error())
		return
	}
	handleResponse(c, h.log, "public stats", http.StatusOK, stats)
}
