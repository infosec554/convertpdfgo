package api

import (
	"github.com/gin-gonic/gin"

	"convertpdfgo/api/handler"
)

// registerFileRoutes mounts /file/* and the cloud-export endpoints.
//
// Upload (POST) is open: the handler enforces tiered caps (20 MB for
// guests, 30 MB once registered). Download is also open at the route
// level — OptionalAuthMiddleware sets user_id when present so the
// handler can do per-owner ACL on owned files, while guests keep
// pulling their guest uploads unchanged.
//
// The rest (/file/list, /file/:id, DELETE, cleanup) requires auth.
//
// Cloud export (/api/storage/* + /oauth/*/callback + /file/:id/export)
// is grouped here because it's a sibling concern to file IO — every
// route in this bucket either reads a file we hold or pushes one to
// the user's external storage account.
func registerFileRoutes(r *gin.Engine, h handler.Handler) {
	// --- Local file IO ----------------------------------------------------
	r.POST("/file/upload", h.UploadFile)
	r.GET("/file/:id/download", h.OptionalAuthMiddleware, h.DownloadFile)

	file := r.Group("/file", h.AuthorizerMiddleware)
	{
		file.GET("/list", h.ListUserFiles)
		file.GET("/:id", h.GetFile)
		file.DELETE("/:id", h.DeleteFile)
		// Note: the admin-only "delete files older than N days" sweep
		// used to live here as /file/cleanup. It moved to
		// /admin/files/cleanup so this group is purely caller-owned
		// operations — see routes_admin.go.
	}

	// --- "Save to ..." cloud-export targets -------------------------------
	//
	// /api/storage/providers stays public so the dropdown can render
	// for guests too (it just won't show any "Connected" providers
	// without a session). The OAuth callback can't sit inside
	// AuthorizerMiddleware because the provider's redirect doesn't
	// carry our JWT — OptionalAuth + an explicit 401 inside the
	// handler when user_id ends up empty is the right shape.
	r.GET("/api/storage/providers", h.OptionalAuthMiddleware, h.ListStorageProviders)
	r.GET("/api/storage/:provider/connect", h.AuthorizerMiddleware, h.ConnectStorageProvider)
	r.DELETE("/api/storage/:provider", h.AuthorizerMiddleware, h.DisconnectStorageProvider)
	r.GET("/oauth/:provider/callback", h.OptionalAuthMiddleware, h.FinishStorageProviderOAuth)
	r.POST("/file/:id/export", h.AuthorizerMiddleware, h.ExportFile)
}
