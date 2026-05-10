package api

import (
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "convertpdfgo/api/docs"
	"convertpdfgo/api/handler"
	"convertpdfgo/config"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/pkg/metrics"
	"convertpdfgo/pkg/middleware"
	"convertpdfgo/service"
	"convertpdfgo/storage"
)

// @title           Auth API
// @version         1.0
// @description     Authentication and Authorization API
// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name Authorization
func New(services service.IServiceManager, log logger.ILogger, cfg config.Config, queue *asynq.Client, redis storage.IRedisStorage) *gin.Engine {
	h := handler.New(services, log, cfg, queue, redis)
	r := gin.Default()

	// === Global middlewares (order matters) ===
	// 1. RequestID first so every later log/metric carries it.
	r.Use(middleware.RequestID())
	// 2. SecurityHeaders / CORS before any handler runs.
	r.Use(middleware.SecurityHeaders())
	r.Use(middleware.CORS(middleware.DefaultCORS()))
	// 3. Hard ceiling on body size — guards memory before multipart parser.
	// Hard ceiling slightly above the per-handler upload cap (registered =
	// 100 MB) so the multipart parser doesn't keep reading past the point
	// where the upload handler will reject anyway. Headroom is for
	// multipart boundaries / form fields.
	r.Use(middleware.BodyLimit(110 << 20)) // 110 MiB
	// 4. gzip after body cap; skip when serving file downloads.
	r.Use(gzip.Gzip(gzip.DefaultCompression, gzip.WithExcludedExtensions([]string{".pdf", ".docx", ".xlsx", ".pptx", ".jpg", ".jpeg", ".png", ".zip"})))
	// 5. Metrics observes the route template, so it must run after Gin
	//    matches the route — Gin runs middleware in registration order
	//    and FullPath() is set before c.Next() in handlers.
	r.Use(middleware.PrometheusMetrics())

	// === Health checks + metrics (no auth, no rate limit) ===
	r.GET("/healthz", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })
	r.GET("/readyz", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ready"}) })
	r.GET("/metrics", gin.WrapH(metrics.Handler()))

	// === Swagger ===
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	r.Use(middleware.RateLimiterMiddleware())

	// === Auth ===
	authGroup := r.Group("/auth")
	{
		// Email + password
		authGroup.POST("/register", h.Register)
		authGroup.POST("/register/confirm", h.RegisterConfirm)
		authGroup.POST("/login", h.Login)
		authGroup.POST("/refresh", h.Refresh)
		authGroup.POST("/logout", h.AuthorizerMiddleware, h.Logout)

		// Passwordless (email + 6-digit OTP)
		authGroup.POST("/login/otp/request", h.OTPLoginRequest)
		authGroup.POST("/login/otp/verify", h.OTPLoginVerify)

		// Password reset
		authGroup.POST("/password/forgot", h.ForgotPassword)
		authGroup.POST("/password/reset", h.ResetPassword)

		// OAuth
		authGroup.GET("/google", h.GoogleLogin)
		authGroup.GET("/google/callback", h.GoogleCallback)
		authGroup.GET("/github", h.GithubLogin)
		authGroup.GET("/github/callback", h.GithubCallback)
	}

	// Legacy OTP shims (kept for backward compatibility).
	r.POST("/otp/send", h.SendOTP)
	r.POST("/otp/confirm", h.ConfirmOTP)

	// === Profile (authenticated) ===
	me := r.Group("/me")
	me.Use(h.AuthorizerMiddleware)
	{
		me.GET("", h.GetMyProfile)
		me.PUT("", h.UpdateMyProfile)
		me.PUT("/password", h.ChangePassword)
		me.GET("/auth-history", h.GetMyAuthHistory)
	}

	// === Admin: legacy + new RBAC routes ===
	// Order: AuthorizerMiddleware (so user_role is set) → AdminAuditMiddleware
	// (so every mutation gets a forensic row) → per-route RequirePermission.
	admin := r.Group("/admin")
	admin.Use(h.AuthorizerMiddleware, h.AdminAuditMiddleware)
	{
		// Legacy: only checked role==admin string. Kept for backward compat.
		admin.GET("/logs/:id", h.AdminMiddleware, h.GetLogsByJobID)

		// User management (RBAC-gated).
		admin.GET("/users", h.RequirePermission("users:read"), h.ListUsersAdmin)
		admin.GET("/users/:id", h.RequirePermission("users:read"), h.GetUserAdmin)
		admin.PUT("/users/:id/role", h.RequirePermission("users:write"), h.UpdateUserRoleAdmin)
		admin.PUT("/users/:id/status", h.RequirePermission("users:write"), h.UpdateUserStatusAdmin)
		admin.DELETE("/users/:id", h.RequirePermission("users:delete"), h.DeleteUserAdmin)
		admin.POST("/users/:id/revoke-tokens", h.RequirePermission("users:write"), h.RevokeUserTokensAdmin)

		// Audit viewer.
		admin.GET("/audit", h.RequirePermission("audit:read"), h.ListAuditAdmin)
		admin.GET("/audit/user/:id", h.RequirePermission("audit:read"), h.AuditByUserAdmin)
		admin.GET("/audit/suspicious", h.RequirePermission("audit:read"), h.SuspiciousLoginsAdmin)

		// Stats dashboard (overview + per-feature drill-downs).
		admin.GET("/stats/overview", h.RequirePermission("stats:read"), h.StatsOverviewAdmin)
		admin.GET("/stats/users", h.RequirePermission("stats:read"), h.StatsUsersByDayAdmin)
		admin.GET("/stats/jobs", h.RequirePermission("stats:read"), h.StatsJobsAdmin)
		admin.GET("/stats/storage", h.RequirePermission("stats:read"), h.StatsStorageAdmin)
		admin.GET("/stats/cache", h.RequirePermission("stats:read"), h.StatsCacheAdmin)

		// Job moderation (Asynq inspector under the hood).
		admin.GET("/jobs/:queue/:state", h.RequirePermission("jobs:read"), h.ListJobsAdmin)
		admin.POST("/jobs/:queue/:id/retry", h.RequirePermission("jobs:moderate"), h.RetryJobAdmin)
		admin.POST("/jobs/:queue/:id/archive", h.RequirePermission("jobs:moderate"), h.ArchiveJobAdmin)
		admin.DELETE("/jobs/:queue/:id", h.RequirePermission("jobs:moderate"), h.DeleteJobAdmin)
	}

	// === Role (adminlar uchun) ===
	role := r.Group("/role")
	role.Use(h.AuthorizerMiddleware, h.AdminMiddleware)
	{
		role.POST("/", h.CreateRole)
		role.PUT("/:id", h.UpdateRole)
		role.GET("/", h.ListRoles)
	}

	stats := r.Group("/stats")
	stats.Use(h.AuthorizerMiddleware)
	{
		stats.GET("/user", h.GetUserStats)
	}

	// === SysUser (admin uchun) ===
	sysuser := r.Group("/sysuser")
	sysuser.Use(h.AuthorizerMiddleware, h.AdminMiddleware)
	{
		sysuser.POST("/", h.CreateSysUser)
	}

	// === Fayllar (token kerak, chunki user_id kerak) ===
	r.POST("/file/upload", h.UploadFile)

	file := r.Group("/file")

	file.Use(h.AuthorizerMiddleware)

	{
		file.GET("/:id", h.GetFile)
		file.DELETE("/:id", h.DeleteFile)
		file.GET("/list", h.ListUserFiles)
		file.GET("/cleanup", h.AdminMiddleware, h.CleanupOldFiles)
	}

	// === PDF xizmatlari (token shart emas — optional auth) ===
	pdf := r.Group("/api/pdf")

	{
		pdf.POST("/merge", h.CreateMergeJob)
		pdf.GET("/merge/:id", h.GetMergeJob)

		pdf.POST("/split", h.CreateSplitJob)
		pdf.GET("/split/:id", h.GetSplitJob)

		pdf.POST("/removepage", h.CreateRemovePagesJob)
		pdf.GET("/removepage/:id", h.GetRemovePagesJob)

		pdf.POST("/extract", h.CreateExtractJob)
		pdf.GET("/extract/:id", h.GetExtractJob)

		pdf.POST("/compress", h.CreateCompressJob)
		pdf.GET("/compress/:id", h.GetCompressJob)

		pdf.POST("/jpg-to-pdf", h.CreateJPGToPDF)
		pdf.GET("/jpg-to-pdf/:id", h.GetJPGToPDFJob)

		pdf.POST("/pdf-to-jpg", h.CreatePDFToJPG)
		pdf.GET("/pdf-to-jpg/:id", h.GetPDFToJPG)

		pdf.POST("/rotate", h.CreateRotateJob)
		pdf.GET("/rotate/:id", h.GetRotateJob)

		pdf.POST("/crop", h.CreateCropJob)
		pdf.GET("/crop/:id", h.GetCropJob)

		pdf.POST("/unlock", h.CreateUnlockJob)
		pdf.GET("/unlock/:id", h.GetUnlockJob)

		pdf.POST("/protect", h.CreateProtectJob)
		pdf.GET("/protect/:id", h.GetProtectJob)

		pdf.POST("/add-page-numbers", h.CreateAddPageNumbersJob)
		pdf.GET("/add-page-numbers/:id", h.GetAddPageNumbersJob)

		pdf.POST("/inspect", h.CreateInspectJob)
		pdf.GET("/inspect/:id", h.GetInspectJob)

		pdf.POST("/header-footer", h.AddHeaderFooter)
		pdf.GET("/header-footer/:id", h.GetHeaderFooterJob)

		pdf.POST("/detect-blank", h.CreateDetectBlankPagesJob)
		pdf.GET("/detect-blank/:id", h.GetDetectBlankPagesJob)

		pdf.POST("/qr-code", h.CreateQRCodeJob)
		pdf.GET("/qr-code/:id", h.GetQRCodeJob)

		pdf.POST("/share", h.CreateSharedLink)
		pdf.GET("/share/:token", h.GetSharedLink)

		pdf.POST("/pdf-to-word", h.CreatePDFToWordJob)
		pdf.GET("/pdf-to-word/:id", h.GetPDFToWordJob)

		pdf.POST("/word-to-pdf", h.CreateWordToPDF)
		pdf.GET("/word-to-pdf/:id", h.GetWordToPDFJob)

		pdf.POST("/excel-to-pdf", h.CreateExcelToPDF)
		pdf.GET("/excel-to-pdf/:id", h.GetExcelToPDFJob)

		pdf.POST("/ppt-to-pdf", h.CreatePowerPointToPDF)
		pdf.GET("/ppt-to-pdf/:id", h.GetPowerPointToPDFJob)
		
		pdf.POST("/html-to-pdf", h.CreateHTMLToPDF)
		pdf.GET("/html-to-pdf/:id", h.GetHTMLToPDFJob)
	}

	return r
}
