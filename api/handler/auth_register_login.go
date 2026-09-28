package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/jwt"
	"convertpdfgo/pkg/timeouts"
)

// Login godoc
// @Router       /auth/login [post]
// @Summary      Email + password login (admin console)
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body models.LoginRequest true "credentials"
// @Success      200 {object} models.AuthTokens
// @Failure      401 {object} models.Response
//
// Operators of the admin console authenticate with a local bcrypt password
// instead of an OAuth provider. Regular site users sign in via Google/GitHub
// and have no password set, so this route rejects them.
func (h Handler) Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleResponse(c, h.log, "invalid request", http.StatusBadRequest, err.Error())
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	user, err := h.services.User().GetForLoginByEmail(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) || user.ID == "" {
		// Generic message + a throwaway bcrypt to keep timing uniform and
		// blunt account enumeration.
		_ = bcrypt.CompareHashAndPassword([]byte("$2a$10$invalidinvalidinvalidinvalidinvalidinvalidinvalidinv"), []byte(req.Password))
		h.audit(c, models.AuthEventLoginFailed, "", email, "password")
		handleResponse(c, h.log, "invalid email or password", http.StatusUnauthorized, nil)
		return
	} else if err != nil {
		handleResponse(c, h.log, "lookup failed", http.StatusInternalServerError, err.Error())
		return
	}

	if user.Status != "" && user.Status != "active" {
		handleResponse(c, h.log, "account not active", http.StatusForbidden, nil)
		return
	}

	if user.Password == "" || bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)) != nil {
		h.audit(c, models.AuthEventLoginFailed, user.ID, email, "password")
		handleResponse(c, h.log, "invalid email or password", http.StatusUnauthorized, nil)
		return
	}

	tokens, err := issueTokens(user.ID, user.Role)
	if err != nil {
		handleResponse(c, h.log, "token error", http.StatusInternalServerError, err.Error())
		return
	}
	h.audit(c, models.AuthEventLogin, user.ID, email, "password")
	handleResponse(c, h.log, "ok", http.StatusOK, tokens)
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

// issueTokens mints an access+refresh pair for an authenticated user. Shared
// by every login path (OAuth, refresh) since the only thing that differs
// across paths is *how* identity is established — token shape is invariant.
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
