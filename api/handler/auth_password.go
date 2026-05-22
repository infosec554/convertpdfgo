package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/pkg/timeouts"
)

// ForgotPassword godoc
// @Router       /auth/password/forgot [post]
// @Summary      Send a password-reset OTP
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body models.ForgotPasswordRequest true "email"
// @Success      200 {object} map[string]string
func (h Handler) ForgotPassword(c *gin.Context) {
	var req models.ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleResponse(c, h.log, "invalid request", http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	// Always respond 200 to avoid leaking which emails are registered.
	exists, _ := h.services.User().ExistsByEmail(ctx, req.Email)
	if !exists {
		h.log.Warning("password-reset for unknown email", logger.String("email", req.Email))
		handleResponse(c, h.log, "if an account exists, an OTP has been sent", http.StatusOK, nil)
		return
	}

	otpID, err := h.services.Otp().SendOtp(ctx, req.Email)
	if err != nil {
		handleResponse(c, h.log, "failed to send OTP", http.StatusInternalServerError, err.Error())
		return
	}
	handleResponse(c, h.log, "OTP sent", http.StatusOK, gin.H{"otp_id": otpID})
}

// ResetPassword godoc
// @Router       /auth/password/reset [post]
// @Summary      Reset password using OTP
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body models.ResetPasswordRequest true "OTP + new password"
// @Success      200 {object} models.Response
// @Failure      401 {object} models.Response
func (h Handler) ResetPassword(c *gin.Context) {
	var req models.ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleResponse(c, h.log, "invalid request", http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	email, savedCode, expiresAt, err := h.services.Otp().GetUnconfirmedByID(ctx, req.OtpID)
	if err != nil {
		handleResponse(c, h.log, "OTP not found", http.StatusUnauthorized, err.Error())
		return
	}
	if time.Now().After(expiresAt) {
		handleResponse(c, h.log, "OTP expired", http.StatusUnauthorized, nil)
		return
	}
	if req.Code != savedCode {
		handleResponse(c, h.log, "incorrect OTP", http.StatusUnauthorized, nil)
		return
	}

	user, err := h.services.User().GetForLoginByEmail(ctx, email)
	if err != nil || user.ID == "" {
		handleResponse(c, h.log, "user not found", http.StatusNotFound, nil)
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		handleResponse(c, h.log, "hash failed", http.StatusInternalServerError, err.Error())
		return
	}
	if err := h.services.User().UpdatePassword(ctx, user.ID, string(hashed)); err != nil {
		handleResponse(c, h.log, "update failed", http.StatusInternalServerError, err.Error())
		return
	}
	if err := h.services.Otp().UpdateStatusToConfirmed(ctx, req.OtpID); err != nil {
		h.log.Warning("password-reset: marking OTP confirmed failed", logger.Error(err))
	}

	h.audit(c, models.AuthEventPasswordReset, user.ID, email, "email")
	handleResponse(c, h.log, "password reset", http.StatusOK, nil)
}

// ChangePassword godoc
// @Router       /me/password [put]
// @Security     ApiKeyAuth
// @Summary      Change current user's password
// @Tags         user
// @Accept       json
// @Produce      json
// @Param        request body models.ChangePasswordRequest true "current+new"
// @Success      200 {object} models.Response
// @Failure      401 {object} models.Response
func (h Handler) ChangePassword(c *gin.Context) {
	var req models.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleResponse(c, h.log, "invalid request", http.StatusBadRequest, err.Error())
		return
	}

	userID := c.GetString("user_id")
	if userID == "" {
		handleResponse(c, h.log, "unauthorized", http.StatusUnauthorized, nil)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	user, err := h.services.User().GetByID(ctx, userID)
	if err != nil || user == nil {
		handleResponse(c, h.log, "user not found", http.StatusNotFound, nil)
		return
	}

	login, err := h.services.User().GetForLoginByEmail(ctx, user.Email)
	if err != nil {
		handleResponse(c, h.log, "lookup failed", http.StatusInternalServerError, err.Error())
		return
	}
	if login.Password == "" {
		handleResponse(c, h.log, "this account has no password (social login)", http.StatusBadRequest, nil)
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(login.Password), []byte(req.CurrentPassword)); err != nil {
		handleResponse(c, h.log, "current password is wrong", http.StatusUnauthorized, nil)
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		handleResponse(c, h.log, "hash failed", http.StatusInternalServerError, err.Error())
		return
	}
	if err := h.services.User().UpdatePassword(ctx, userID, string(hashed)); err != nil {
		handleResponse(c, h.log, "update failed", http.StatusInternalServerError, err.Error())
		return
	}
	h.audit(c, models.AuthEventPasswordChange, userID, user.Email, "email")
	handleResponse(c, h.log, "password changed", http.StatusOK, nil)
}
