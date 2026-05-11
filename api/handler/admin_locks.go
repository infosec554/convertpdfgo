package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/timeouts"
)

// ListLockedAccountsAdmin godoc
// @Router       /admin/auth/locked [get]
// @Security     ApiKeyAuth
// @Summary      Currently locked-out accounts (admin)
// @Description  Reads the loginlimiter's Redis keyspace via SCAN. Returns
//
//	every email that has hit the failure threshold and has
//	not yet expired. Use /admin/auth/unlock to clear one.
//
// @Tags         admin-auth
// @Param        limit query int false "max accounts to return, default 200"
// @Success      200 {object} models.Response
func (h Handler) ListLockedAccountsAdmin(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	rows, err := h.loginLimit.ListLocked(ctx, 200)
	if err != nil {
		handleResponse(c, h.log, "failed to read lockouts", http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]models.LockedAccount, 0, len(rows))
	for _, e := range rows {
		out = append(out, models.LockedAccount{Email: e.Email, FailCount: e.FailCount})
	}
	handleResponse(c, h.log, "locked accounts", http.StatusOK, gin.H{
		"items": out,
		"count": len(out),
	})
}

// UnlockAccountAdmin godoc
// @Router       /admin/auth/unlock [post]
// @Security     ApiKeyAuth
// @Summary      Clear an account lockout (admin)
// @Description  Wipes the loginlimiter counter for the given email so the
//
//	next attempt goes through bcrypt again. Idempotent —
//	unlocking an already-unlocked account is a no-op.
//
// @Tags         admin-auth
// @Param        request body object{email=string} true "email to unlock"
// @Success      200 {object} models.Response
func (h Handler) UnlockAccountAdmin(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		handleResponse(c, h.log, "invalid body", http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	if err := h.loginLimit.Unlock(ctx, req.Email); err != nil {
		handleResponse(c, h.log, "failed to unlock", http.StatusInternalServerError, err.Error())
		return
	}
	h.audit(c, "admin_auth_unlock", c.GetString("user_id"), "", req.Email)
	handleResponse(c, h.log, "account unlocked", http.StatusOK, gin.H{"email": req.Email})
}
