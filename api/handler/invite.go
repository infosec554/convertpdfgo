// Admin invite handlers.
//
//	POST /admin/users/invite     (auth, users:write)
//	GET  /auth/invite/:token     (public, preview screen)
//	POST /auth/accept-invite     (public, sets password + auto-login)
package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/jwt"
	"convertpdfgo/pkg/timeouts"
)

// InviteAdminCreate godoc
// @Summary  Send an admin invite email to a new (or existing pending) user
// @Tags     admin-users
// @Accept   json
// @Produce  json
// @Param    request body models.InviteAdminRequest true "invite"
// @Success  202 {object} map[string]string
// @Router   /admin/users/invite [post]
// @Security ApiKeyAuth
func (h Handler) InviteAdminCreate(c *gin.Context) {
	var req models.InviteAdminRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleResponse(c, h.log, "invalid request", http.StatusBadRequest, err.Error())
		return
	}
	invitedBy := c.GetString("user_id")
	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	token, err := h.services.Invite().Create(ctx, req, invitedBy)
	if err != nil {
		handleResponse(c, h.log, "invite failed", http.StatusBadRequest, err.Error())
		return
	}
	// Return the token too so the admin UI can show "Copy link" when
	// the email path is misconfigured. The real recipient sees it
	// inside the email; logged here it's a one-time bypass for the
	// inviter, not a credential leak.
	handleResponse(c, h.log, "invite queued", http.StatusAccepted, gin.H{
		"email": req.Email,
		"role":  req.Role,
		"token": token,
	})
}

// InvitePreview godoc
// @Summary  Resolve an invite token to email + role for the accept screen
// @Tags     auth
// @Produce  json
// @Param    token path string true "Invite token"
// @Success  200 {object} models.InvitePreview
// @Router   /auth/invite/{token} [get]
func (h Handler) InvitePreview(c *gin.Context) {
	token := c.Param("token")
	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()
	preview, err := h.services.Invite().Preview(ctx, token)
	if err != nil {
		handleResponse(c, h.log, "invite invalid", http.StatusNotFound, err.Error())
		return
	}
	handleResponse(c, h.log, "ok", http.StatusOK, preview)
}

// AcceptInvite godoc
// @Summary  Finish an invite — set password, activate, auto-login
// @Tags     auth
// @Accept   json
// @Produce  json
// @Param    request body models.AcceptInviteRequest true "accept-invite"
// @Success  200 {object} models.RefreshTokenResponse
// @Router   /auth/accept-invite [post]
func (h Handler) AcceptInvite(c *gin.Context) {
	var req models.AcceptInviteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleResponse(c, h.log, "invalid request", http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	userID, role, err := h.services.Invite().Accept(ctx, req)
	if err != nil {
		handleResponse(c, h.log, "accept failed", http.StatusBadRequest, err.Error())
		return
	}

	// Mint the same access/refresh pair /auth/login returns so the
	// browser can jump straight into the admin dashboard without
	// asking for a second login.
	access, refresh, err := jwt.GenerateJWT(map[string]interface{}{
		"user_id":   userID,
		"user_role": role,
	})
	if err != nil {
		handleResponse(c, h.log, "token mint failed", http.StatusInternalServerError, err.Error())
		return
	}
	handleResponse(c, h.log, "welcome", http.StatusOK, models.RefreshTokenResponse{
		AccessToken:  access,
		RefreshToken: refresh,
	})
}
