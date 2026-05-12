package api

import (
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"convertpdfgo/api/handler"
	"convertpdfgo/pkg/metrics"
)

// registerInfraRoutes wires endpoints that need to live ABOVE the
// rate limiter — health probes, swagger, metrics scrape target, the
// service-cached /api/stats/public counter. None of these touch
// user state and all are idempotent, so blocking them on abuse
// counters would only hurt monitoring.
func registerInfraRoutes(r *gin.Engine, h handler.Handler) {
	r.GET("/healthz", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })
	r.GET("/readyz", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ready"}) })
	r.GET("/metrics", gin.WrapH(metrics.Handler()))
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	r.GET("/api/stats/public", h.GetPublicStats)
}
