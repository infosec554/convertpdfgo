package api

import (
	"github.com/gin-gonic/gin"

	"convertpdfgo/api/handler"
)

// registerAuthRoutes mounts /auth/* — registration, login (email +
// password), password reset, OAuth (google + github), and the public
// side of the admin-invite flow.
//
// /logout is the only auth-required endpoint here; everything else
// is intentionally pre-login.
func registerAuthRoutes(r *gin.Engine, h handler.Handler) {
	auth := r.Group("/auth")
	{
		// Email + password.
		auth.POST("/register", h.Register)
		auth.POST("/login", h.Login)
		auth.POST("/refresh", h.Refresh)
		auth.POST("/logout", h.AuthorizerMiddleware, h.Logout)

		// Password reset.
		auth.POST("/password/forgot", h.ForgotPassword)
		auth.POST("/password/reset", h.ResetPassword)

		// Admin invite acceptance — the token in the URL proves the
		// link is legitimate, so these are public. Preview lets the
		// accept screen show "Invited as admin (you@example.com)"
		// without leaking the underlying user_id.
		auth.GET("/invite/:token", h.InvitePreview)
		auth.POST("/accept-invite", h.AcceptInvite)

		// OAuth.
		auth.GET("/google", h.GoogleLogin)
		auth.GET("/google/callback", h.GoogleCallback)
		auth.GET("/github", h.GithubLogin)
		auth.GET("/github/callback", h.GithubCallback)
	}
}
