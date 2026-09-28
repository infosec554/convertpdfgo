package api

import (
	"github.com/gin-gonic/gin"

	"convertpdfgo/api/handler"
)

// registerAuthRoutes mounts /auth/* — OAuth (google + github) sign-in,
// token refresh + logout, and the public side of the admin-invite flow.
//
// User-facing sign-up is OAuth-only; there is no email+password path
// any more. The "logout" endpoint is the only one here that demands a
// valid session.
func registerAuthRoutes(r *gin.Engine, h handler.Handler) {
	auth := r.Group("/auth")
	{
		auth.POST("/refresh", h.Refresh)
		auth.POST("/logout", h.AuthorizerMiddleware, h.Logout)

		// Email + password login — admin console operators only. Regular
		// users have no password (OAuth-only), so this rejects them.
		auth.POST("/login", h.Login)

		// Admin invite acceptance — the token in the URL proves the
		// link is legitimate, so these are public. Preview lets the
		// accept screen show "Invited as admin (you@example.com)"
		// without leaking the underlying user_id.
		auth.GET("/invite/:token", h.InvitePreview)
		auth.POST("/accept-invite", h.AcceptInvite)

		// OAuth — the only sign-up + sign-in path for end users.
		auth.GET("/google", h.GoogleLogin)
		auth.GET("/google/callback", h.GoogleCallback)
		auth.GET("/github", h.GithubLogin)
		auth.GET("/github/callback", h.GithubCallback)
	}
}
