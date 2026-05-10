package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"convertpdfgo/api/models"
)

// @Router       /api/pdf/share [POST]
// @Summary      Faylga ulashish havolasi yaratish
// @Description  PDF faylni boshqalar bilan ulashish uchun link yaratish
// @Tags         share
// @Accept       json
// @Produce      json
// @Param        request body models.CreateSharedLinkRequest true "File ID and expiration date"
// @Success      201 {object} map[string]string
// @Failure      400 {object} models.Response
// @Failure      500 {object} models.Response
func (h *Handler) CreateSharedLink(c *gin.Context) {
	var req models.CreateSharedLinkRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		handleResponse(c, h.log, "invalid request", http.StatusBadRequest, err.Error())
		return
	}

	if req.FileID == "" {
		handleResponse(c, h.log, "missing file_id", http.StatusBadRequest, nil)
		return
	}

	// Agar expires_at bo‘sh bo‘lsa, 24 soat qo‘shib o‘rnatish
	if req.ExpiresAt == nil {
		defaultExpiry := time.Now().Add(24 * time.Hour)
		req.ExpiresAt = &defaultExpiry
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	token, err := h.services.SharedLink().Create(ctx, req)
	if err != nil {
		handleResponse(c, h.log, "failed to create shared link", http.StatusInternalServerError, err.Error())
		return
	}

	handleResponse(c, h.log, "shared link created", http.StatusCreated, gin.H{"token": token})
}

// @Router       /api/pdf/share/{token} [GET]
// @Summary      Ulashilgan faylni olish
// @Description  Token orqali faylga kirish
// @Tags         share
// @Produce      json
// @Param        token path string true "Shared link token"
// @Success      200 {object} models.SharedLink
// @Failure      404 {object} models.Response
// @Failure      500 {object} models.Response
func (h *Handler) GetSharedLink(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		handleResponse(c, h.log, "missing token", http.StatusBadRequest, nil)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	link, err := h.services.SharedLink().GetByToken(ctx, token)
	if err != nil {
		handleResponse(c, h.log, "shared link not found", http.StatusNotFound, err.Error())
		return
	}

	handleResponse(c, h.log, "shared link retrieved", http.StatusOK, link)
}
