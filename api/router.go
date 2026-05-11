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

// New wires up the HTTP router. Routes are organised top-down by audience:
//
//  1. Infra        — health, metrics, swagger        (no auth, no rate limit)
//  2. Auth         — register/login/logout/oauth     (public + rate limited)
//  3. Profile      — /me/*                           (user-auth)
//  4. Files        — uploads & downloads             (mixed: upload is open,
//     ops require auth)
//  5. PDF jobs     — /api/pdf/* grouped by category  (optional auth)
//  6. Admin        — /admin/*                        (RBAC + audit)
//
// Within a group, related routes are kept adjacent so adding a new
// endpoint is a one-line edit and the file reads top-to-bottom.
//
// @title           convertpdfgo API
// @version         1.0
// @description     PDF processing & authentication API
// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name Authorization
func New(services service.IServiceManager, log logger.ILogger, cfg config.Config, queue *asynq.Client, redis storage.IRedisStorage) *gin.Engine {
	h := handler.New(services, log, cfg, queue, redis)
	r := gin.Default()

	// === Global middlewares (registration order = execution order) ===
	r.Use(middleware.RequestID())                    // 1. tag every request
	r.Use(middleware.SecurityHeaders())              // 2. defensive headers
	r.Use(middleware.CORS(middleware.DefaultCORS())) // 3. CORS preflight
	r.Use(middleware.BodyLimit(110 << 20))           // 4. 110 MiB hard cap
	r.Use(gzip.Gzip(gzip.DefaultCompression,         // 5. compress JSON only
		gzip.WithExcludedExtensions([]string{".pdf", ".docx", ".xlsx", ".pptx", ".jpg", ".jpeg", ".png", ".zip"})))
	r.Use(middleware.PrometheusMetrics()) // 6. record latency

	// =====================================================================
	// 1. Infra (no auth, no rate limit, no audit)
	// =====================================================================
	r.GET("/healthz", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })
	r.GET("/readyz", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ready"}) })
	r.GET("/metrics", gin.WrapH(metrics.Handler()))
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Everything below this line is rate-limited.
	r.Use(middleware.RateLimiterMiddleware())

	// =====================================================================
	// 2. Auth — public endpoints (logout requires a valid token)
	// =====================================================================
	auth := r.Group("/auth")
	{
		// Email + password.
		auth.POST("/register", h.Register)
		auth.POST("/register/confirm", h.RegisterConfirm)
		auth.POST("/login", h.Login)
		auth.POST("/refresh", h.Refresh)
		auth.POST("/logout", h.AuthorizerMiddleware, h.Logout)

		// Passwordless (email + 6-digit OTP).
		auth.POST("/login/otp/request", h.OTPLoginRequest)
		auth.POST("/login/otp/verify", h.OTPLoginVerify)

		// Password reset.
		auth.POST("/password/forgot", h.ForgotPassword)
		auth.POST("/password/reset", h.ResetPassword)

		// OAuth.
		auth.GET("/google", h.GoogleLogin)
		auth.GET("/google/callback", h.GoogleCallback)
		auth.GET("/github", h.GithubLogin)
		auth.GET("/github/callback", h.GithubCallback)
	}

	// =====================================================================
	// 3. Profile — current user (/me/*)
	// =====================================================================
	me := r.Group("/me", h.AuthorizerMiddleware)
	{
		me.GET("", h.GetMyProfile)
		me.PUT("", h.UpdateMyProfile)
		me.PUT("/password", h.ChangePassword)
		me.GET("/auth-history", h.GetMyAuthHistory)
		me.GET("/stats", h.GetUserStats)
	}

	// =====================================================================
	// 4. Files — upload is open (limit-based register prompt); the rest
	// needs auth.
	// =====================================================================
	r.POST("/file/upload", h.UploadFile) // open: tiered cap inside handler
	// Download is open in the sense that no auth is *required* — but
	// OptionalAuthMiddleware populates user_id when present so the
	// handler's ownership check can recognise registered owners. Guests
	// keep accessing their guest uploads as before.
	r.GET("/file/:id/download", h.OptionalAuthMiddleware, h.DownloadFile)
	file := r.Group("/file", h.AuthorizerMiddleware)
	{
		file.GET("/list", h.ListUserFiles)
		file.GET("/:id", h.GetFile)
		file.DELETE("/:id", h.DeleteFile)
		file.GET("/cleanup", h.AdminMiddleware, h.CleanupOldFiles) // admin-only
	}

	// =====================================================================
	// 5. PDF jobs (/api/pdf/*) — optional auth (guests may use within
	// limits). Endpoints come in pairs: POST creates+enqueues a job,
	// GET fetches its status. Sub-grouped by intent.
	// =====================================================================
	pdf := r.Group("/api/pdf")
	{
		// 5a. Page operations (rearrange / shrink content of an existing PDF).
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

		// 5b. Visual transforms (geometry, decorations).
		pdf.POST("/rotate", h.CreateRotateJob)
		pdf.GET("/rotate/:id", h.GetRotateJob)
		pdf.POST("/crop", h.CreateCropJob)
		pdf.GET("/crop/:id", h.GetCropJob)
		pdf.POST("/add-page-numbers", h.CreateAddPageNumbersJob)
		pdf.GET("/add-page-numbers/:id", h.GetAddPageNumbersJob)
		pdf.POST("/header-footer", h.AddHeaderFooter)
		pdf.GET("/header-footer/:id", h.GetHeaderFooterJob)
		pdf.POST("/qr-code", h.CreateQRCodeJob)
		pdf.GET("/qr-code/:id", h.GetQRCodeJob)

		// 5c. Security (encrypt / decrypt).
		pdf.POST("/protect", h.CreateProtectJob)
		pdf.GET("/protect/:id", h.GetProtectJob)
		pdf.POST("/unlock", h.CreateUnlockJob)
		pdf.GET("/unlock/:id", h.GetUnlockJob)

		// 5d. Inspection / analysis.
		pdf.POST("/inspect", h.CreateInspectJob)
		pdf.GET("/inspect/:id", h.GetInspectJob)
		pdf.POST("/detect-blank", h.CreateDetectBlankPagesJob)
		pdf.GET("/detect-blank/:id", h.GetDetectBlankPagesJob)

		// 5e. Image ↔ PDF.
		pdf.POST("/jpg-to-pdf", h.CreateJPGToPDF)
		pdf.GET("/jpg-to-pdf/:id", h.GetJPGToPDFJob)
		pdf.POST("/pdf-to-jpg", h.CreatePDFToJPG)
		pdf.GET("/pdf-to-jpg/:id", h.GetPDFToJPG)

		// 5f. Office ↔ PDF (Gotenberg).
		pdf.POST("/word-to-pdf", h.CreateWordToPDF)
		pdf.GET("/word-to-pdf/:id", h.GetWordToPDFJob)
		pdf.POST("/pdf-to-word", h.CreatePDFToWordJob)
		pdf.GET("/pdf-to-word/:id", h.GetPDFToWordJob)
		pdf.POST("/excel-to-pdf", h.CreateExcelToPDF)
		pdf.GET("/excel-to-pdf/:id", h.GetExcelToPDFJob)
		pdf.POST("/ppt-to-pdf", h.CreatePowerPointToPDF)
		pdf.GET("/ppt-to-pdf/:id", h.GetPowerPointToPDFJob)
		pdf.POST("/html-to-pdf", h.CreateHTMLToPDF)
		pdf.GET("/html-to-pdf/:id", h.GetHTMLToPDFJob)

		// 5h. Phase 1 features (PDF_FEATURES.md): watermark, extract-text,
		// n-up, url-to-pdf, metadata.
		pdf.POST("/watermark", h.CreateWatermarkJob)
		pdf.GET("/watermark/:id", h.GetWatermarkJob)
		pdf.POST("/extract-text", h.CreateExtractTextJob)
		pdf.GET("/extract-text/:id", h.GetExtractTextJob)
		pdf.POST("/nup", h.CreateNUpJob)
		pdf.GET("/nup/:id", h.GetNUpJob)
		pdf.POST("/url-to-pdf", h.CreateURLToPDFJob)
		pdf.GET("/url-to-pdf/:id", h.GetURLToPDFJob)
		pdf.POST("/metadata", h.CreateMetadataJob)
		pdf.GET("/metadata/:id", h.GetMetadataJob)

		// 5g. Sharing (public link with a token, no auth on GET).
		pdf.POST("/share", h.CreateSharedLink)
		pdf.GET("/share/:token", h.GetSharedLink)
	}

	// =====================================================================
	// 6. Admin (/admin/*) — RBAC, audited. Auth + AdminAuditMiddleware
	// run for everything in this group; per-route RequirePermission
	// declares the specific capability needed.
	// =====================================================================
	admin := r.Group("/admin", h.AuthorizerMiddleware, h.AdminAuditMiddleware)
	{
		// 6a. Users.
		users := admin.Group("/users")
		{
			users.GET("", h.RequirePermission("users:read"), h.ListUsersAdmin)
			users.GET("/:id", h.RequirePermission("users:read"), h.GetUserAdmin)
			users.PUT("/:id/role", h.RequirePermission("users:write"), h.UpdateUserRoleAdmin)
			users.PUT("/:id/status", h.RequirePermission("users:write"), h.UpdateUserStatusAdmin)
			users.DELETE("/:id", h.RequirePermission("users:delete"), h.DeleteUserAdmin)
			users.POST("/:id/restore", h.RequirePermission("users:write"), h.RestoreUserAdmin)
			users.POST("/:id/reset-password", h.RequirePermission("users:write"), h.ResetUserPasswordAdmin)
			users.POST("/:id/revoke-tokens", h.RequirePermission("users:write"), h.RevokeUserTokensAdmin)
		}

		// 6a.1 Files (cross-user listing + force delete).
		files := admin.Group("/files")
		{
			files.GET("", h.RequirePermission("files:read"), h.ListAllFilesAdmin)
			files.DELETE("/:id", h.RequirePermission("files:delete"), h.DeleteFileAdmin)
		}

		// 6a.2 Auth lockouts — read keyspace via SCAN, manually unlock.
		authAdmin := admin.Group("/auth")
		{
			authAdmin.GET("/locked", h.RequirePermission("audit:read"), h.ListLockedAccountsAdmin)
			authAdmin.POST("/unlock", h.RequirePermission("users:write"), h.UnlockAccountAdmin)
		}

		// 6b. Sysusers (system operators, separate from end users).
		sysuser := admin.Group("/sysusers")
		{
			sysuser.POST("", h.RequirePermission("sysusers:write"), h.CreateSysUser)
		}

		// 6c. Roles.
		roles := admin.Group("/roles")
		{
			roles.GET("", h.RequirePermission("roles:read"), h.ListRoles)
			roles.POST("", h.RequirePermission("roles:write"), h.CreateRole)
			roles.PUT("/:id", h.RequirePermission("roles:write"), h.UpdateRole)
		}

		// 6d. Audit log.
		audit := admin.Group("/audit", h.RequirePermission("audit:read"))
		{
			audit.GET("", h.ListAuditAdmin)
			audit.GET("/recent", h.RecentActivityAdmin)
			audit.GET("/user/:id", h.AuditByUserAdmin)
			audit.GET("/suspicious", h.SuspiciousLoginsAdmin)
		}

		// 6e. Stats dashboard.
		stats := admin.Group("/stats", h.RequirePermission("stats:read"))
		{
			stats.GET("/overview", h.StatsOverviewAdmin)
			stats.GET("/users", h.StatsUsersByDayAdmin)
			stats.GET("/jobs", h.StatsJobsAdmin)
			stats.GET("/storage", h.StatsStorageAdmin)
			stats.GET("/cache", h.StatsCacheAdmin)
		}

		// 6f. Job moderation (Asynq inspector).
		jobs := admin.Group("/jobs")
		{
			jobs.GET("/:queue/:state", h.RequirePermission("jobs:read"), h.ListJobsAdmin)
			jobs.POST("/:queue/:id/retry", h.RequirePermission("jobs:moderate"), h.RetryJobAdmin)
			jobs.POST("/:queue/:id/archive", h.RequirePermission("jobs:moderate"), h.ArchiveJobAdmin)
			jobs.DELETE("/:queue/:id", h.RequirePermission("jobs:moderate"), h.DeleteJobAdmin)
		}

		// 6g. Per-job logs (legacy AdminMiddleware — old "role==admin" check).
		admin.GET("/logs/:id", h.AdminMiddleware, h.GetLogsByJobID)
	}

	return r
}
