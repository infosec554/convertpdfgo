// Package handler — legacy /otp/send and /otp/confirm endpoints are kept
// thin shims around the consolidated /auth/login/otp flow. New code should
// hit /auth/login/otp/request and /auth/login/otp/verify directly.
package handler

import "github.com/gin-gonic/gin"

// SendOTP godoc
// @Router       /otp/send [post]
// @Summary      DEPRECATED — use /auth/login/otp/request
// @Tags         otp
// @Accept       json
// @Produce      json
// @Param        request body models.OTPLoginRequest true "email"
// @Success      200  {object}  map[string]string
func (h Handler) SendOTP(c *gin.Context) {
	h.OTPLoginRequest(c)
}

// ConfirmOTP godoc
// @Router       /otp/confirm [post]
// @Summary      DEPRECATED — use /auth/login/otp/verify
// @Tags         otp
// @Accept       json
// @Produce      json
// @Param        request body models.OTPLoginVerifyRequest true "OTP"
// @Success      200  {object}  models.AuthTokens
func (h Handler) ConfirmOTP(c *gin.Context) {
	h.OTPLoginVerify(c)
}
