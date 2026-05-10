package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// GetUserStats godoc
// @Summary      Foydalanuvchi statistikasi
// @Description  Foydalanuvchining PDF bo‘yicha statistikasi (birlashtirish, bo‘lish va h.k.)
// @Tags         stats
// @Accept       json
// @Produce      json
// @Success      200 {object} models.UserStats
// @Failure      401 {object} models.Response
// @Failure      500 {object} models.Response
// @Router       /api/stats/user [get]
// @Security     ApiKeyAuth
func (h Handler) GetUserStats(c *gin.Context) {
	userID := c.GetString("user_id") // JWT middleware orqali olingan

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stats, err := h.services.Stat().GetUserStats(ctx, userID)
	if err != nil {
		handleResponse(c, h.log, "failed to get user stats", http.StatusInternalServerError, err.Error())
		return
	}

	handleResponse(c, h.log, "user stats", http.StatusOK, stats)
}
