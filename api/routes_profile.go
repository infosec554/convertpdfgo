package api

import (
	"github.com/gin-gonic/gin"

	"convertpdfgo/api/handler"
)

// registerProfileRoutes mounts /me/* — the calling user's own
// profile, password change, auth history, and per-user usage stats.
// Every route requires a valid JWT.
func registerProfileRoutes(r *gin.Engine, h handler.Handler) {
	me := r.Group("/me", h.AuthorizerMiddleware)
	{
		me.GET("", h.GetMyProfile)
		me.PUT("", h.UpdateMyProfile)
		me.PUT("/password", h.ChangePassword)
		me.GET("/auth-history", h.GetMyAuthHistory)
		me.GET("/stats", h.GetUserStats)
	}
}
