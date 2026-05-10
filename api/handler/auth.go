package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/jwt"
)

// audit fires a best-effort write to auth_audit. Used by login/logout/etc.
func (h Handler) audit(c *gin.Context, event, userID, email, provider string) {
	var uidPtr *string
	if userID != "" {
		uidPtr = &userID
	}
	h.services.Audit().Record(c.Request.Context(), models.AuthAudit{
		UserID:    uidPtr,
		Email:     email,
		Event:     event,
		Provider:  provider,
		IP:        c.ClientIP(),
		UserAgent: c.GetHeader("User-Agent"),
	})
}

// AuthorizerMiddleware extracts the bearer token, validates the signature,
// rejects revoked JTIs, and stuffs user_id / user_role into the request
// context for downstream handlers.
func (h Handler) AuthorizerMiddleware(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header missing"})
		return
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid Authorization format. Use 'Bearer {token}'"})
		return
	}

	claims, err := jwt.ExtractClaims(parts[1])
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
		return
	}

	// Only access tokens authenticate API calls. Refresh tokens belong only
	// to /auth/refresh.
	if typ, _ := claims["typ"].(string); typ != "" && typ != jwt.TokenTypeAccess {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "wrong token type"})
		return
	}

	jti, _ := claims["jti"].(string)
	if jti != "" && h.bl.IsRevoked(c.Request.Context(), jti) {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token revoked"})
		return
	}

	userID, ok := claims["user_id"].(string)
	if !ok || userID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "user_id not found in token"})
		return
	}

	// Per-user revocation: admin "force logout" or password change writes
	// a cut-off timestamp. Any token whose iat is older than the cut-off
	// is rejected, regardless of its JTI.
	if cutoff := h.bl.UserRevokedAfter(c.Request.Context(), userID); cutoff > 0 {
		var iat int64
		switch v := claims["iat"].(type) {
		case float64:
			iat = int64(v)
		case int64:
			iat = v
		}
		if iat > 0 && iat < cutoff {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "session revoked; please log in again"})
			return
		}
	}

	c.Set("user_id", userID)
	if jti != "" {
		c.Set("jti", jti)
	}
	if exp, ok := claims["exp"]; ok {
		c.Set("exp", exp)
	}
	if role, ok := claims["user_role"].(string); ok {
		c.Set("user_role", role)
	}

	c.Next()
}

func (h Handler) AdminMiddleware(c *gin.Context) {
	roleVal, exists := c.Get("user_role")
	if !exists {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "user_role missing"})
		return
	}
	role, ok := roleVal.(string)
	if !ok || role != "admin" {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "only admin access"})
		return
	}
	c.Next()
}
