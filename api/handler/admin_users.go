package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/jwt"
	"convertpdfgo/pkg/timeouts"
)

// ListUsersAdmin godoc
// @Router       /admin/users [get]
// @Security     ApiKeyAuth
// @Summary      List users (admin)
// @Tags         admin-users
// @Param        q      query string false "search by email or name"
// @Param        role   query string false "filter by role"
// @Param        status query string false "filter by status"
// @Param        limit  query int    false "page size (default 50, max 200)"
// @Param        offset query int    false "page offset"
// @Success      200 {object} models.Response
// @Failure      403 {object} models.Response
func (h Handler) ListUsersAdmin(c *gin.Context) {
	var f models.UserListFilter
	if err := c.ShouldBindQuery(&f); err != nil {
		handleResponse(c, h.log, "invalid query", http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	users, total, err := h.services.User().List(ctx, f)
	if err != nil {
		handleResponse(c, h.log, "failed to list users", http.StatusInternalServerError, err.Error())
		return
	}

	handleResponse(c, h.log, "users listed", http.StatusOK, gin.H{
		"items":  users,
		"total":  total,
		"limit":  f.Limit,
		"offset": f.Offset,
	})
}

// GetUserAdmin godoc
// @Router       /admin/users/{id} [get]
// @Security     ApiKeyAuth
// @Summary      Get a user by id (admin)
// @Tags         admin-users
// @Param        id path string true "user id"
// @Success      200 {object} models.User
func (h Handler) GetUserAdmin(c *gin.Context) {
	id := c.Param("id")
	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	user, err := h.services.User().GetByID(ctx, id)
	if err != nil {
		handleResponse(c, h.log, "user not found", http.StatusNotFound, err.Error())
		return
	}
	handleResponse(c, h.log, "user", http.StatusOK, user)
}

// UpdateUserRoleAdmin godoc
// @Router       /admin/users/{id}/role [put]
// @Security     ApiKeyAuth
// @Summary      Change a user's role (admin)
// @Tags         admin-users
// @Param        id path string true "user id"
// @Param        body body models.UpdateUserRoleRequest true "new role"
// @Success      200 {object} models.Response
func (h Handler) UpdateUserRoleAdmin(c *gin.Context) {
	id := c.Param("id")

	var req models.UpdateUserRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleResponse(c, h.log, "invalid body", http.StatusBadRequest, err.Error())
		return
	}

	// Only superadmins can grant superadmin / admin roles. Otherwise an
	// "admin" with users:write could trivially escalate.
	actorRole := c.GetString("user_role")
	if (req.Role == "superadmin" || req.Role == "admin") && actorRole != "superadmin" {
		handleResponse(c, h.log, "only superadmin can grant admin/superadmin", http.StatusForbidden, nil)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	if err := h.services.User().UpdateRole(ctx, id, req.Role); err != nil {
		handleResponse(c, h.log, "failed to update role", http.StatusInternalServerError, err.Error())
		return
	}

	h.audit(c, "admin_user_role_change", c.GetString("user_id"), id, req.Role)
	handleResponse(c, h.log, "role updated", http.StatusOK, gin.H{"id": id, "role": req.Role})
}

// UpdateUserStatusAdmin godoc
// @Router       /admin/users/{id}/status [put]
// @Security     ApiKeyAuth
// @Summary      Change a user's status (admin)
// @Tags         admin-users
// @Param        id path string true "user id"
// @Param        body body models.UpdateUserStatusRequest true "new status"
// @Success      200 {object} models.Response
func (h Handler) UpdateUserStatusAdmin(c *gin.Context) {
	id := c.Param("id")

	var req models.UpdateUserStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleResponse(c, h.log, "invalid body", http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	if err := h.services.User().UpdateStatus(ctx, id, req.Status); err != nil {
		handleResponse(c, h.log, "failed to update status", http.StatusInternalServerError, err.Error())
		return
	}

	h.audit(c, "admin_user_status_change", c.GetString("user_id"), id, req.Status)
	handleResponse(c, h.log, "status updated", http.StatusOK, gin.H{"id": id, "status": req.Status})
}

// DeleteUserAdmin godoc
// @Router       /admin/users/{id} [delete]
// @Security     ApiKeyAuth
// @Summary      Soft-delete a user (admin)
// @Tags         admin-users
// @Param        id path string true "user id"
// @Success      200 {object} models.Response
func (h Handler) DeleteUserAdmin(c *gin.Context) {
	id := c.Param("id")
	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	if err := h.services.User().UpdateStatus(ctx, id, "deleted"); err != nil {
		handleResponse(c, h.log, "failed to delete user", http.StatusInternalServerError, err.Error())
		return
	}

	h.audit(c, "admin_user_delete", c.GetString("user_id"), id, "")
	handleResponse(c, h.log, "user deleted", http.StatusOK, gin.H{"id": id})
}

// RevokeUserTokensAdmin godoc
// @Router       /admin/users/{id}/revoke-tokens [post]
// @Security     ApiKeyAuth
// @Summary      Force-logout a user (revoke every issued token, admin)
// @Tags         admin-users
// @Param        id path string true "user id"
// @Success      200 {object} models.Response
// @Failure      403 {object} models.Response
func (h Handler) RevokeUserTokensAdmin(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		handleResponse(c, h.log, "missing user id", http.StatusBadRequest, nil)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	// TTL must outlive the longest possible refresh token, otherwise an
	// old token could come back from the dead once the gate expires.
	if err := h.bl.RevokeUser(ctx, id, jwt.RefreshTokenTTL+time.Hour); err != nil {
		handleResponse(c, h.log, "failed to revoke", http.StatusInternalServerError, err.Error())
		return
	}

	h.audit(c, "admin_user_revoke_tokens", c.GetString("user_id"), id, "")
	handleResponse(c, h.log, "tokens revoked", http.StatusOK, gin.H{"id": id})
}

// RestoreUserAdmin godoc
// @Router       /admin/users/{id}/restore [post]
// @Security     ApiKeyAuth
// @Summary      Restore a soft-deleted user (admin)
// @Description  Flips status from 'deleted' back to 'active'. Idempotent —
//
//	restoring an already-active user is a no-op success.
//
// @Tags         admin-users
// @Param        id path string true "user id"
// @Success      200 {object} models.Response
func (h Handler) RestoreUserAdmin(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		handleResponse(c, h.log, "missing user id", http.StatusBadRequest, nil)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	if err := h.services.User().UpdateStatus(ctx, id, "active"); err != nil {
		handleResponse(c, h.log, "failed to restore user", http.StatusInternalServerError, err.Error())
		return
	}
	h.audit(c, "admin_user_restore", c.GetString("user_id"), id, "")
	handleResponse(c, h.log, "user restored", http.StatusOK, gin.H{"id": id, "status": "active"})
}

// ResetUserPasswordAdmin godoc
// @Router       /admin/users/{id}/reset-password [post]
// @Security     ApiKeyAuth
// @Summary      Trigger a password-reset OTP for a user (admin)
// @Description  Generates an OTP and emails it via the same flow the user
//
//	would self-service. Admin doesn't see the OTP — they only
//	kick off delivery.
//
// @Tags         admin-users
// @Param        id path string true "user id"
// @Success      200 {object} models.Response
func (h Handler) ResetUserPasswordAdmin(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		handleResponse(c, h.log, "missing user id", http.StatusBadRequest, nil)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.Process)
	defer cancel()

	user, err := h.services.User().GetByID(ctx, id)
	if err != nil || user == nil {
		handleResponse(c, h.log, "user not found", http.StatusNotFound, nil)
		return
	}
	if user.Email == "" {
		handleResponse(c, h.log, "user has no email", http.StatusBadRequest, nil)
		return
	}

	otpID, err := h.services.Otp().SendOtp(ctx, user.Email)
	if err != nil {
		handleResponse(c, h.log, "failed to send reset OTP", http.StatusInternalServerError, err.Error())
		return
	}

	h.audit(c, "admin_user_reset_password", c.GetString("user_id"), id, "")
	handleResponse(c, h.log, "reset OTP sent", http.StatusOK, gin.H{
		"user_id": id,
		"email":   user.Email,
		"otp_id":  otpID,
	})
}
