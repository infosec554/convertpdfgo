package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/timeouts"
)

// GetMyProfile godoc
// @Router       /me [get]
// @Security     ApiKeyAuth
// @Summary      Get the authenticated user's profile
// @Tags         user
// @Produce      json
// @Success      200 {object} models.User
// @Failure      401 {object} models.Response
func (h *Handler) GetMyProfile(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		handleResponse(c, h.log, "unauthorized", http.StatusUnauthorized, nil)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	user, err := h.services.User().GetByID(ctx, userID)
	if err != nil {
		handleResponse(c, h.log, "failed to get user", http.StatusInternalServerError, err.Error())
		return
	}
	handleResponse(c, h.log, "user profile", http.StatusOK, user)
}

// GetMyAuthHistory godoc
// @Router       /me/auth-history [get]
// @Security     ApiKeyAuth
// @Summary      Recent auth events for the current user (login, logout, ...)
// @Tags         user
// @Produce      json
// @Success      200 {array} models.AuthAudit
// @Failure      401 {object} models.Response
func (h *Handler) GetMyAuthHistory(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		handleResponse(c, h.log, "unauthorized", http.StatusUnauthorized, nil)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	events, err := h.services.Audit().ListByUser(ctx, userID, 50)
	if err != nil {
		handleResponse(c, h.log, "failed to fetch history", http.StatusInternalServerError, err.Error())
		return
	}
	handleResponse(c, h.log, "auth history", http.StatusOK, events)
}

// UpdateMyProfile godoc
// @Router       /me [put]
// @Security     ApiKeyAuth
// @Summary      Update the authenticated user's profile (name)
// @Tags         user
// @Accept       json
// @Produce      json
// @Param        request body models.UpdateProfileRequest true "profile"
// @Success      200 {object} models.Response
// @Failure      401 {object} models.Response
func (h *Handler) UpdateMyProfile(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		handleResponse(c, h.log, "unauthorized", http.StatusUnauthorized, nil)
		return
	}

	var req models.UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleResponse(c, h.log, "invalid request", http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	if err := h.services.User().UpdateProfile(ctx, userID, req.Name); err != nil {
		handleResponse(c, h.log, "update failed", http.StatusInternalServerError, err.Error())
		return
	}
	handleResponse(c, h.log, "profile updated", http.StatusOK, gin.H{"id": userID, "name": req.Name})
}

// ChangeMyPassword godoc
// @Router       /me/password [put]
// @Security     ApiKeyAuth
// @Summary      Change the authenticated user's password
// @Tags         user
// @Accept       json
// @Produce      json
// @Param        request body models.ChangePasswordRequest true "passwords"
// @Success      200 {object} models.Response
// @Failure      401 {object} models.Response
func (h *Handler) ChangeMyPassword(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		handleResponse(c, h.log, "unauthorized", http.StatusUnauthorized, nil)
		return
	}

	var req models.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleResponse(c, h.log, "invalid request", http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	// Resolve the caller's email, then fetch the record that carries the
	// password hash (GetByID does not select it).
	me, err := h.services.User().GetByID(ctx, userID)
	if err != nil {
		handleResponse(c, h.log, "failed to load user", http.StatusInternalServerError, err.Error())
		return
	}
	full, err := h.services.User().GetForLoginByEmail(ctx, me.Email)
	if err != nil || full.Password == "" {
		handleResponse(c, h.log, "no password is set for this account", http.StatusBadRequest, nil)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(full.Password), []byte(req.CurrentPassword)) != nil {
		handleResponse(c, h.log, "current password is incorrect", http.StatusUnauthorized, nil)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		handleResponse(c, h.log, "could not hash password", http.StatusInternalServerError, err.Error())
		return
	}
	if err := h.services.User().UpdatePassword(ctx, userID, string(hash)); err != nil {
		handleResponse(c, h.log, "update failed", http.StatusInternalServerError, err.Error())
		return
	}
	handleResponse(c, h.log, "password changed", http.StatusOK, gin.H{"id": userID})
}
