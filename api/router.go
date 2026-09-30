// Package api wires up the HTTP surface. Route registration is split
// across small register*Routes files — one per audience — so adding
// a new endpoint means touching one short, focused file instead of a
// 300-line wall.
//
// Audience layout:
//
//	routes_infra.go    health / ready / metrics / swagger / public-stats
//	                   (above the rate limiter — monitoring + landing)
//	routes_auth.go     /auth/* — register, login, OAuth, invite-accept
//	routes_profile.go  /me/* — calling user's own profile + stats
//	routes_files.go    /file/* + /api/storage/* + /oauth/*/callback
//	                   + /file/:id/export
//	routes_pdf.go      /api/pdf/* — every conversion / OCR endpoint
//	routes_admin.go    /admin/* — RBAC + audited
//
// New() owns the middleware chain and the order in which the four
// public sub-systems get registered. Everything below
// `RateLimiterMiddleware` is subject to the 5-rps + 24-h-ban policy.
package api

import (
	"context"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"

	_ "convertpdfgo/api/docs"
	"convertpdfgo/api/handler"
	"convertpdfgo/config"
	"convertpdfgo/pkg/accesslog"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/pkg/middleware"
	"convertpdfgo/pkg/notify"
	"convertpdfgo/pkg/wshub"
	"convertpdfgo/service"
	"convertpdfgo/storage"
)

// New wires up the HTTP router. Each register*Routes call covers one
// audience and is independently readable.
//
// @title           convertpdfgo API
// @version         1.0
// @description     PDF processing & authentication API
// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name Authorization
func New(services service.IServiceManager, log logger.ILogger, cfg config.Config, queue *asynq.Client, redis storage.IRedisStorage, hub *wshub.Hub, bus notify.Bus) *gin.Engine {
	h := handler.New(services, log, cfg, queue, redis, hub, bus)
	r := gin.Default()

	// --- Global middlewares (order = execution order on each request) ----
	r.Use(middleware.RequestID())                    // tag every request
	r.Use(middleware.SecurityHeaders())              // defensive headers
	r.Use(middleware.CORS(middleware.DefaultCORS())) // CORS preflight
	r.Use(middleware.BodyLimit(110 << 20))           // 110 MiB hard cap
	r.Use(gzip.Gzip(gzip.DefaultCompression,         // compress JSON only
		gzip.WithExcludedExtensions([]string{".pdf", ".docx", ".xlsx", ".pptx", ".jpg", ".jpeg", ".png", ".zip"})))
	r.Use(middleware.PrometheusMetrics()) // latency histogram

	// Traffic log: record every request (IP, action, account/guest) for the
	// admin console. Self-contained pool + table; best-effort, non-blocking.
	accesslog.Init(context.Background(), cfg, log)
	r.Use(accesslog.Middleware())

	// --- Above the rate limiter — monitoring + landing ------------------
	registerInfraRoutes(r, h)

	// Everything below this line is rate-limited (5 rps sustained,
	// burst 10, Redis-backed 24-hour ban on abuse).
	r.Use(middleware.RateLimiterMiddleware(redis))

	registerAuthRoutes(r, h)
	registerProfileRoutes(r, h)
	registerFileRoutes(r, h)
	registerPDFRoutes(r, h)
	registerContactRoutes(r, h)
	registerChatRoutes(r, h)
	registerAIRoutes(r, h)
	registerCVRoutes(r, h)
	registerBuildRoutes(r, h)
	registerAdminRoutes(r, h)

	// WebSocket endpoints. Auth lives in the handler (token via query
	// param) since browsers can't send Authorization headers in the
	// upgrade request. Both routes belong here below the rate limiter
	// so they share IP-level abuse protection.
	r.GET("/ws", h.WS)
	r.GET("/ws/admin", h.WSAdmin)

	return r
}
