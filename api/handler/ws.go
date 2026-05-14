package handler

import (
	"net/http"

	"github.com/coder/websocket"
	"github.com/gin-gonic/gin"

	"convertpdfgo/pkg/jwt"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/pkg/wshub"
)

// WS godoc
// @Router       /ws [GET]
// @Summary      Open a WebSocket for live notifications (user-scoped)
// @Description  Auth via JWT in the `token` query parameter (browsers can't
// @Description  send custom Authorization headers in the WS handshake).
// @Description  Receives job-status updates for the authenticated user.
// @Tags         realtime
// @Param        token query string true "JWT access token"
// @Success      101 {string} string "Switching Protocols"
// @Failure      401 {object} models.Response
func (h Handler) WS(c *gin.Context) {
	uid, role, ok := h.authenticateWSToken(c)
	if !ok {
		return
	}
	h.acceptWS(c, uid, role == "admin")
}

// WSAdmin godoc
// @Router       /ws/admin [GET]
// @Security     ApiKeyAuth
// @Summary      Open a WebSocket for admin-only notifications
// @Description  Same auth as /ws but rejects non-admin tokens. Subscribes
// @Description  to admin broadcasts (new signups, suspicious logins,
// @Description  worker failures, contact submissions).
// @Tags         realtime
// @Param        token query string true "JWT access token (admin)"
// @Success      101 {string} string "Switching Protocols"
// @Failure      403 {object} models.Response
func (h Handler) WSAdmin(c *gin.Context) {
	uid, role, ok := h.authenticateWSToken(c)
	if !ok {
		return
	}
	if role != "admin" {
		handleResponse(c, h.log, "admin only", http.StatusForbidden, nil)
		return
	}
	h.acceptWS(c, uid, true)
}

// authenticateWSToken pulls the JWT from ?token=, validates it the same
// way AuthorizerMiddleware would (signature + access-typ + blacklist +
// per-user revocation), and returns the user id and role.
func (h Handler) authenticateWSToken(c *gin.Context) (userID, role string, ok bool) {
	token := c.Query("token")
	if token == "" {
		handleResponse(c, h.log, "token required", http.StatusUnauthorized, nil)
		return "", "", false
	}
	claims, err := jwt.ExtractClaims(token)
	if err != nil {
		handleResponse(c, h.log, "invalid token", http.StatusUnauthorized, nil)
		return "", "", false
	}
	if typ, _ := claims["typ"].(string); typ != "" && typ != jwt.TokenTypeAccess {
		handleResponse(c, h.log, "wrong token type", http.StatusUnauthorized, nil)
		return "", "", false
	}
	jti, _ := claims["jti"].(string)
	if jti != "" && h.bl.IsRevoked(c.Request.Context(), jti) {
		handleResponse(c, h.log, "token revoked", http.StatusUnauthorized, nil)
		return "", "", false
	}
	uid, _ := claims["user_id"].(string)
	if uid == "" {
		handleResponse(c, h.log, "user_id missing", http.StatusUnauthorized, nil)
		return "", "", false
	}
	role, _ = claims["user_role"].(string)
	return uid, role, true
}

// acceptWS upgrades the connection, registers the client with the Hub,
// and runs the read/write pumps until the peer goes away. Hub
// (un)register is bracketed around Run so a crashed Run never leaks
// a registry entry.
func (h Handler) acceptWS(c *gin.Context, userID string, isAdmin bool) {
	if h.hub == nil {
		// Misconfigured deployment — Hub wasn't wired in main.go. Fail
		// loudly rather than silently accepting and never delivering.
		handleResponse(c, h.log, "ws hub not initialised", http.StatusServiceUnavailable, nil)
		return
	}

	// InsecureSkipVerify=true on the Origin check is intentional: this
	// API serves browser clients from arbitrary origins (mobile webview,
	// PWA, third-party embeds). The OAuth/JWT layer is the actual
	// authentication boundary, not Origin. Production deployments
	// behind a known frontend can tighten this later via a custom
	// AcceptOptions.OriginPatterns.
	conn, err := websocket.Accept(c.Writer, c.Request, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		// Accept already wrote a 4xx/5xx; nothing more to do.
		h.log.Warning("ws: accept failed", logger.Error(err))
		return
	}

	client := wshub.NewClient(conn, userID, isAdmin)
	h.hub.Register(client)
	defer h.hub.Unregister(client)

	client.Run(c.Request.Context())
}
