package api

import (
	"github.com/gin-gonic/gin"

	"convertpdfgo/api/handler"
)

// registerAdminRoutes mounts /admin/* — every operator-facing
// endpoint. The whole group is wrapped with AuthorizerMiddleware
// + AdminAuditMiddleware so each request lands in admin_audit, and
// per-route RequirePermission declares the capability needed.
//
// Sub-groups (one block per concern, ordered roughly by frequency
// of use in the dashboard):
//
//   /admin/users    — list, role / status / lifecycle, invites
//   /admin/files    — cross-user listing + force delete
//   /admin/auth     — login-lockout viewer + manual unlock
//   /admin/roles    — role CRUD (role_permissions seed lives in SQL)
//   /admin/audit    — auth_audit reader
//   /admin/stats    — dashboard aggregates
//   /admin/jobs     — Asynq inspector (retry / archive / delete)
//   /admin/logs/:id — per-job logs (legacy "role==admin" check)
func registerAdminRoutes(r *gin.Engine, h handler.Handler) {
	admin := r.Group("/admin", h.AuthorizerMiddleware, h.AdminAuditMiddleware)

	// --- Users ------------------------------------------------------------
	users := admin.Group("/users")
	{
		users.GET("", h.RequirePermission("users:read"), h.ListUsersAdmin)
		users.GET("/:id", h.RequirePermission("users:read"), h.GetUserAdmin)
		users.PUT("/:id/role", h.RequirePermission("users:write"), h.UpdateUserRoleAdmin)
		users.PUT("/:id/status", h.RequirePermission("users:write"), h.UpdateUserStatusAdmin) // block / unblock / soft-delete
		users.DELETE("/:id", h.RequirePermission("users:delete"), h.DeleteUserAdmin)
		users.POST("/:id/restore", h.RequirePermission("users:write"), h.RestoreUserAdmin)
		users.POST("/:id/reset-password", h.RequirePermission("users:write"), h.ResetUserPasswordAdmin)
		users.POST("/:id/revoke-tokens", h.RequirePermission("users:write"), h.RevokeUserTokensAdmin)
		// Invite a new admin (or regular user) by email. Creates the
		// user row as 'pending' and stashes a Redis-tokenised accept
		// link (48 h TTL); the invitee finishes onboarding via the
		// public /auth/invite/{token} + /auth/accept-invite pair.
		users.POST("/invite", h.RequirePermission("users:write"), h.InviteAdminCreate)
	}

	// --- Files (cross-user) ----------------------------------------------
	files := admin.Group("/files")
	{
		files.GET("", h.RequirePermission("files:read"), h.ListAllFilesAdmin)
		files.DELETE("/:id", h.RequirePermission("files:delete"), h.DeleteFileAdmin)
	}

	// --- Auth lockouts (login_limiter) -----------------------------------
	authAdmin := admin.Group("/auth")
	{
		authAdmin.GET("/locked", h.RequirePermission("audit:read"), h.ListLockedAccountsAdmin)
		authAdmin.POST("/unlock", h.RequirePermission("users:write"), h.UnlockAccountAdmin)
	}

	// --- Roles ------------------------------------------------------------
	// The two-tier user|admin model lives at the column level; this
	// group is the leftover surface for the (currently unused) named-
	// role CRUD. Kept for future extensibility.
	roles := admin.Group("/roles")
	{
		roles.GET("", h.RequirePermission("roles:read"), h.ListRoles)
		roles.POST("", h.RequirePermission("roles:write"), h.CreateRole)
		roles.PUT("/:id", h.RequirePermission("roles:write"), h.UpdateRole)
	}

	// --- Audit log --------------------------------------------------------
	audit := admin.Group("/audit", h.RequirePermission("audit:read"))
	{
		audit.GET("", h.ListAuditAdmin)
		audit.GET("/recent", h.RecentActivityAdmin)
		audit.GET("/user/:id", h.AuditByUserAdmin)
		audit.GET("/suspicious", h.SuspiciousLoginsAdmin)
	}

	// --- Stats dashboard --------------------------------------------------
	stats := admin.Group("/stats", h.RequirePermission("stats:read"))
	{
		stats.GET("/overview", h.StatsOverviewAdmin)
		stats.GET("/users", h.StatsUsersByDayAdmin)
		stats.GET("/jobs", h.StatsJobsAdmin)
		stats.GET("/storage", h.StatsStorageAdmin)
		stats.GET("/cache", h.StatsCacheAdmin)
	}

	// --- Job moderation (Asynq inspector) --------------------------------
	jobs := admin.Group("/jobs")
	{
		jobs.GET("/:queue/:state", h.RequirePermission("jobs:read"), h.ListJobsAdmin)
		jobs.POST("/:queue/:id/retry", h.RequirePermission("jobs:moderate"), h.RetryJobAdmin)
		jobs.POST("/:queue/:id/archive", h.RequirePermission("jobs:moderate"), h.ArchiveJobAdmin)
		jobs.DELETE("/:queue/:id", h.RequirePermission("jobs:moderate"), h.DeleteJobAdmin)
	}

	// --- Per-job logs (legacy AdminMiddleware — "role==admin") -----------
	admin.GET("/logs/:id", h.AdminMiddleware, h.GetLogsByJobID)
}
