package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// RequirePermission returns a middleware that aborts with 403 unless the
// caller's role grants `perm`. Pair with AuthorizerMiddleware so user_role
// is already in the gin context.
//
// Usage:
//
//	admin := r.Group("/admin")
//	admin.Use(h.AuthorizerMiddleware, h.RequirePermission("users:read"))
//	admin.GET("/users", h.ListUsers)
func (h Handler) RequirePermission(perm string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get("user_role")
		roleStr, _ := role.(string)
		if roleStr == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "user_role missing"})
			return
		}
		if !h.services.Permission().HasPermission(c.Request.Context(), roleStr, perm) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":      "permission denied",
				"required":   perm,
				"role":       roleStr,
				"request_id": c.GetString("request_id"),
			})
			return
		}
		c.Next()
	}
}

// RequireAnyPermission allows access when the caller has at least one of
// the listed perms. Useful for read-only screens that span multiple
// permission groups.
func (h Handler) RequireAnyPermission(perms ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get("user_role")
		roleStr, _ := role.(string)
		if roleStr == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "user_role missing"})
			return
		}
		ctx := c.Request.Context()
		for _, p := range perms {
			if h.services.Permission().HasPermission(ctx, roleStr, p) {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error":         "permission denied",
			"required_any":  perms,
			"role":          roleStr,
			"request_id":    c.GetString("request_id"),
		})
	}
}
