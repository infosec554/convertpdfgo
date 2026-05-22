package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
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
// @Description  Creates an active user and returns auth tokens. No email verification step.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body models.RegisterRequest true "register"
// @Success      201 {object} models.AuthTokens
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

	userID, err := h.services.User().Create(ctx, models.CreateUser{
		Name:     req.Name,
		Email:    req.Email,
		Password: string(hashed),
		Provider: "email",
	})
	if err != nil {
		handleResponse(c, h.log, "user creation failed", http.StatusInternalServerError, err.Error())
		return
	}

	tokens, err := issueTokens(userID, "user")
	if err != nil {
		handleResponse(c, h.log, "token error", http.StatusInternalServerError, err.Error())
		return
	}

	h.audit(c, models.AuthEventRegister, userID, req.Email, "email")
	h.publishAdminEvent(ctx, notify.TypeUserRegistered, notify.UserRegistered{
		UserID: userID,
		Email:  req.Email,
	})
	h.log.Info("user registered", logger.String("email", req.Email), logger.String("user_id", userID))
	handleResponse(c, h.log, "registered", http.StatusCreated, tokens)
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

