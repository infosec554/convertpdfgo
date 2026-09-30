package api

import (
	"github.com/gin-gonic/gin"

	"convertpdfgo/api/handler"
)

// registerAIRoutes mounts /api/ai/* — the command bar's router.
//
// Public, like the tools it routes to: someone typing "merge these"
// into the box on the homepage has not signed in, and asking them to
// before the site will even say which button to press would be a
// strange way to introduce a free tool.
//
// OptionalAuthMiddleware so a signed-in caller is still identified,
// which is what rate limiting per account would need later.
func registerAIRoutes(r *gin.Engine, h handler.Handler) {
	ai := r.Group("/api/ai", h.OptionalAuthMiddleware)
	{
		ai.POST("/command", h.AICommand)
	}
}
