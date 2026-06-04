package api

import (
	"github.com/gin-gonic/gin"

	"convertpdfgo/api/handler"
)

// registerBuildRoutes mounts /api/build/* — AI-driven generators that
// produce new files from a prompt rather than transforming an upload.
// First member of this family: the PowerPoint builder.
func registerBuildRoutes(r *gin.Engine, h handler.Handler) {
	build := r.Group("/api/build", h.OptionalAuthMiddleware)
	{
		build.POST("/pptx", h.CreatePPTXBuilderJob)
		build.GET("/pptx/:id", h.GetPPTXBuilderJob)
		build.POST("/xlsx", h.CreateXLSXBuilderJob)
		build.GET("/xlsx/:id", h.GetXLSXBuilderJob)
	}
}
