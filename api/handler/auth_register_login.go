package handler

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/jwt"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/pkg/notify"
	"convertpdfgo/pkg/timeouts"
)

// Register godoc
// @Router       /auth/register [post]
// @Summary      Register a new account (email + password)
// @Description  Creates an unverified user and emails a 6-digit OTP. Call /auth/register/confirm with the OTP to activate.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body models.RegisterRequest true "register"
// @Success      201 {object} models.RegisterResponse
// @Failure      400 {object} models.Response
// @Failure      409 {object} models.Response
// @Failure      500 {object} models.Response
func (h Handler) Register(c *gin.Context) {
	var req models.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleResponse(c, h.log, "invalid request", http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	exists, err := h.services.User().ExistsByEmail(ctx, req.Email)
	if err != nil {
		handleResponse(c, h.log, "lookup failed", http.StatusInternalServerError, err.Error())
		return
	}
	if exists {
		handleResponse(c, h.log, "email already registered", http.StatusConflict, nil)
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		handleResponse(c, h.log, "hash failed", http.StatusInternalServerError, err.Error())
		return
	}

	if _, err := h.services.User().Create(ctx, models.CreateUser{
		Name:     req.Name,
		Email:    req.Email,
		Password: string(hashed),
		Provider: "email",
	}); err != nil {
		handleResponse(c, h.log, "user creation failed", http.StatusInternalServerError, err.Error())
		return
	}

	otpID, err := h.services.Otp().SendOtp(ctx, req.Email)
	if err != nil {
		handleResponse(c, h.log, "failed to send OTP", http.StatusInternalServerError, err.Error())
		return
	}

	h.audit(c, models.AuthEventRegister, "", req.Email, "email")
	h.log.Info("user registered, OTP sent", logger.String("email", req.Email))
	handleResponse(c, h.log, "registration started; verify via OTP", http.StatusCreated, models.RegisterResponse{
		OtpID:   otpID,
		Message: "We sent a 6-digit code to " + req.Email,
	})
}

// RegisterConfirm godoc
// @Router       /auth/register/confirm [post]
// @Summary      Verify email by OTP and finalise registration
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body models.RegisterConfirmRequest true "OTP confirmation"
// @Success      200 {object} models.AuthTokens
// @Failure      400 {object} models.Response
// @Failure      401 {object} models.Response
// @Failure      500 {object} models.Response
func (h Handler) RegisterConfirm(c *gin.Context) {
	var req models.RegisterConfirmRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleResponse(c, h.log, "invalid request", http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	tokens, err := h.verifyOTPAndIssueTokens(ctx, req.OtpID, req.Code)
	if err != nil {
		writeOTPError(c, h.log, err)
		return
	}
	handleResponse(c, h.log, "account verified", http.StatusOK, tokens)
}

// Login godoc
// @Router       /auth/login [post]
// @Summary      Login with email + password
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body models.EmailPasswordLoginRequest true "credentials"
// @Success      200 {object} models.AuthTokens
// @Failure      400 {object} models.Response
// @Failure      401 {object} models.Response
func (h Handler) Login(c *gin.Context) {
	var req models.EmailPasswordLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleResponse(c, h.log, "invalid request", http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	// Brute-force shield: too many recent fails for this email → 429.
	if err := h.loginLimit.Check(ctx, req.Email); err != nil {
		// Admin gets a real-time signal so they can investigate the source.
		h.publishAdminEvent(ctx, notify.TypeSuspiciousAuth, notify.SuspiciousAuth{
			Email:     req.Email,
			IP:        c.ClientIP(),
			UserAgent: c.GetHeader("User-Agent"),
		})
		handleResponse(c, h.log, "account temporarily locked", http.StatusTooManyRequests, nil)
		return
	}

	user, err := h.services.User().GetForLoginByEmail(ctx, req.Email)
	if err != nil || user.ID == "" {
		h.loginLimit.RecordFailure(ctx, req.Email)
		h.audit(c, models.AuthEventLoginFail, "", req.Email, "email")
		handleResponse(c, h.log, "invalid credentials", http.StatusUnauthorized, nil)
		return
	}
	if user.Status != "active" {
		handleResponse(c, h.log, "account not active", http.StatusForbidden, nil)
		return
	}
	if user.Password == "" {
		handleResponse(c, h.log, "account uses social login; sign in with the linked provider", http.StatusUnauthorized, nil)
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		h.loginLimit.RecordFailure(ctx, req.Email)
		h.audit(c, models.AuthEventLoginFail, user.ID, req.Email, "email")
		handleResponse(c, h.log, "invalid credentials", http.StatusUnauthorized, nil)
		return
	}

	h.loginLimit.Reset(ctx, req.Email)

	role := user.Role
	if role == "" {
		role = "user"
	}
	tokens, err := issueTokens(user.ID, role)
	if err != nil {
		handleResponse(c, h.log, "token error", http.StatusInternalServerError, err.Error())
		return
	}
	h.audit(c, models.AuthEventLoginOK, user.ID, req.Email, "email")
	h.log.Info("login ok", logger.String("user_id", user.ID), logger.String("role", role))
	handleResponse(c, h.log, "login ok", http.StatusOK, tokens)
}

// Refresh godoc
// @Router       /auth/refresh [post]
// @Summary      Exchange refresh token for a new access+refresh pair
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body models.RefreshRequest true "refresh"
// @Success      200 {object} models.AuthTokens
// @Failure      401 {object} models.Response
func (h Handler) Refresh(c *gin.Context) {
	var req models.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleResponse(c, h.log, "invalid request", http.StatusBadRequest, err.Error())
		return
	}

	claims, err := jwt.ParseToken(req.RefreshToken)
	if err != nil {
		handleResponse(c, h.log, "invalid refresh token", http.StatusUnauthorized, err.Error())
		return
	}
	if t, _ := claims["typ"].(string); t != jwt.TokenTypeRefresh {
		handleResponse(c, h.log, "wrong token type", http.StatusUnauthorized, nil)
		return
	}

	jti, _ := claims["jti"].(string)
	if jti != "" && h.bl.IsRevoked(c.Request.Context(), jti) {
		handleResponse(c, h.log, "refresh token revoked", http.StatusUnauthorized, nil)
		return
	}

	userID, _ := claims["user_id"].(string)
	role, _ := claims["user_role"].(string)
	if userID == "" {
		handleResponse(c, h.log, "missing user_id", http.StatusUnauthorized, nil)
		return
	}

	// Rotate: revoke the old refresh JTI before issuing a new pair.
	if jti != "" {
		_ = h.bl.Revoke(c.Request.Context(), jti, jwt.ExpiresAt(claims))
	}

	tokens, err := issueTokens(userID, role)
	if err != nil {
		handleResponse(c, h.log, "token error", http.StatusInternalServerError, err.Error())
		return
	}
	handleResponse(c, h.log, "refreshed", http.StatusOK, tokens)
}

// Logout godoc
// @Router       /auth/logout [post]
// @Security     ApiKeyAuth
// @Summary      Revoke the current access token
// @Tags         auth
// @Produce      json
// @Success      200 {object} models.Response
// @Failure      401 {object} models.Response
func (h Handler) Logout(c *gin.Context) {
	jti, _ := c.Get("jti")
	expVal, _ := c.Get("exp")

	jtiStr, _ := jti.(string)
	if jtiStr == "" {
		handleResponse(c, h.log, "no token id", http.StatusUnauthorized, nil)
		return
	}

	expiresAt := time.Now().Add(jwt.AccessTokenTTL)
	if v, ok := expVal.(float64); ok {
		expiresAt = time.Unix(int64(v), 0)
	}

	if err := h.bl.Revoke(c.Request.Context(), jtiStr, expiresAt); err != nil {
		handleResponse(c, h.log, "logout failed", http.StatusInternalServerError, err.Error())
		return
	}
	uid, _ := c.Get("user_id")
	uidStr, _ := uid.(string)
	h.audit(c, models.AuthEventLogout, uidStr, "", "")
	handleResponse(c, h.log, "logged out", http.StatusOK, nil)
}

// === Helpers ===

func issueTokens(userID, role string) (models.AuthTokens, error) {
	if role == "" {
		role = "user"
	}
	access, refresh, err := jwt.GenerateJWT(map[string]interface{}{
		"user_id":   userID,
		"user_role": role,
	})
	if err != nil {
		return models.AuthTokens{}, err
	}
	return models.AuthTokens{AccessToken: access, RefreshToken: refresh}, nil
}

// verifyOTPAndIssueTokens checks the OTP, marks it confirmed, ensures the
// user exists, and returns a token pair. Used by both registration confirm
// and OTP-based passwordless login.
func (h Handler) verifyOTPAndIssueTokens(ctx context.Context, otpID, code string) (models.AuthTokens, error) {
	email, savedCode, expiresAt, err := h.services.Otp().GetUnconfirmedByID(ctx, otpID)
	if err != nil {
		return models.AuthTokens{}, ErrOTPNotFound
	}
	if time.Now().After(expiresAt) {
		return models.AuthTokens{}, ErrOTPExpired
	}
	if code != savedCode {
		return models.AuthTokens{}, ErrOTPMismatch
	}
	if err := h.services.Otp().UpdateStatusToConfirmed(ctx, otpID); err != nil {
		return models.AuthTokens{}, err
	}

	user, err := h.services.User().GetForLoginByEmail(ctx, email)
	userID := user.ID
	role := user.Role
	if errors.Is(err, pgx.ErrNoRows) || userID == "" {
		newID, createErr := h.services.User().Create(ctx, models.CreateUser{
			Email:    email,
			Provider: "otp",
		})
		if createErr != nil {
			return models.AuthTokens{}, createErr
		}
		userID = newID
		role = "user"
		// Best-effort admin notification — a failure here can't roll
		// back the just-created user so we log and move on.
		h.publishAdminEvent(ctx, notify.TypeUserRegistered, notify.UserRegistered{
			UserID: newID,
			Email:  email,
		})
	} else if err != nil {
		return models.AuthTokens{}, err
	} else if user.Status != "" && user.Status != "active" {
		return models.AuthTokens{}, ErrAccountInactive
	}
	if role == "" {
		role = "user"
	}
	return issueTokens(userID, role)
}

// === Sentinel errors for OTP flow ===

var (
	ErrOTPNotFound     = errors.New("otp not found")
	ErrOTPExpired      = errors.New("otp expired")
	ErrOTPMismatch     = errors.New("otp mismatch")
	ErrAccountInactive = errors.New("account not active")
)

func writeOTPError(c *gin.Context, log logger.ILogger, err error) {
	switch {
	case errors.Is(err, ErrOTPNotFound):
		handleResponse(c, log, "OTP not found", http.StatusUnauthorized, nil)
	case errors.Is(err, ErrOTPExpired):
		handleResponse(c, log, "OTP expired", http.StatusUnauthorized, nil)
	case errors.Is(err, ErrOTPMismatch):
		handleResponse(c, log, "incorrect OTP", http.StatusUnauthorized, nil)
	case errors.Is(err, ErrAccountInactive):
		handleResponse(c, log, "account not active", http.StatusForbidden, nil)
	default:
		handleResponse(c, log, "OTP verification failed", http.StatusInternalServerError, err.Error())
	}
}
