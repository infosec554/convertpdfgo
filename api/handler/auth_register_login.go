package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/jwt"
)

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
