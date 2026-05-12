// Handlers for the "Save to ..." cloud-export feature.
//
//   GET  /api/storage/providers                — list configured + connected
//   GET  /api/storage/:provider/connect        — kick off OAuth (302 to provider)
//   GET  /oauth/:provider/callback             — provider returns here with code
//   POST /file/:id/export                      — upload our file to provider
//   DELETE /api/storage/:provider              — revoke our stored token
//
// CSRF on the OAuth flow is enforced with a short-lived Redis key holding
// the random state value the connect step generates; the callback only
// accepts a state that's still in Redis and tied to the same user.
package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/pkg/timeouts"
)

const oauthStateTTL = 10 * time.Minute

// stateKey returns the Redis key we stash the OAuth `state` token in
// while the user is on the provider's consent screen. Embedding the
// user_id stops a stolen state from being redeemed by another
// session.
func stateKey(userID, state string) string {
	return "oauth_state:" + userID + ":" + state
}

// --- GET /api/storage/providers -------------------------------------------

// ListStorageProviders returns one row per provider with capability +
// connection flags. Authenticated; unauthed callers get the
// "Configured" flag only.
func (h Handler) ListStorageProviders(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()
	uid := c.GetString("user_id")
	info := h.services.StorageExport().Providers(ctx, uid)
	handleResponse(c, h.log, "providers", http.StatusOK, info)
}

// --- GET /api/storage/:provider/connect ----------------------------------

// ConnectStorageProvider starts the OAuth dance — generates a state
// token, stashes it in Redis under the user, and 302s the browser at
// the provider's consent screen.
func (h Handler) ConnectStorageProvider(c *gin.Context) {
	provider := c.Param("provider")
	uid := c.GetString("user_id")
	if uid == "" {
		handleResponse(c, h.log, "auth required", http.StatusUnauthorized, nil)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	authURL, state, err := h.services.StorageExport().AuthURL(provider)
	if err != nil {
		handleResponse(c, h.log, "provider not configured", http.StatusBadRequest, err.Error())
		return
	}
	// Persist state so the callback can verify it. Best-effort; if
	// Redis is down the OAuth dance still completes but loses CSRF
	// protection. Surface as a warning rather than failing the user.
	if err := h.redis.SetX(ctx, stateKey(uid, state), "1", oauthStateTTL); err != nil {
		h.log.Warning("oauth state set", logger.Error(err))
	}
	c.Redirect(http.StatusFound, authURL)
}

// --- GET /oauth/:provider/callback ----------------------------------------

// FinishStorageProviderOAuth handles the redirect back from the
// provider — exchanges the `code` for a token and stores it. The
// `state` parameter must match what we stashed in Redis.
func (h Handler) FinishStorageProviderOAuth(c *gin.Context) {
	provider := c.Param("provider")
	uid := c.GetString("user_id")
	if uid == "" {
		handleResponse(c, h.log, "auth required (re-log in and reconnect)", http.StatusUnauthorized, nil)
		return
	}
	code := c.Query("code")
	state := c.Query("state")
	if code == "" || state == "" {
		handleResponse(c, h.log, "missing code or state", http.StatusBadRequest, nil)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	// State check — short-circuits before we ever hit the provider's
	// token endpoint, so a stolen code can't be replayed against us.
	if val, err := h.redis.Get(ctx, stateKey(uid, state)); err != nil || val != "1" {
		handleResponse(c, h.log, "invalid or expired state", http.StatusBadRequest, nil)
		return
	}
	_ = h.redis.Del(ctx, stateKey(uid, state))

	if _, err := h.services.StorageExport().FinishOAuth(ctx, uid, provider, code); err != nil {
		handleResponse(c, h.log, "oauth failed", http.StatusBadRequest, err.Error())
		return
	}
	handleResponse(c, h.log, "connected", http.StatusOK, gin.H{"provider": provider})
}

// --- POST /file/:id/export ------------------------------------------------

// ExportFile uploads a file we hold to the user's chosen provider.
// Synchronous on purpose — typical conversion outputs are small
// enough (<10 MB) that the round-trip fits inside a single HTTP
// request. If we start exporting larger artefacts, switch to the
// async queue model.
func (h Handler) ExportFile(c *gin.Context) {
	fileID := c.Param("id")
	uid := c.GetString("user_id")
	if uid == "" {
		handleResponse(c, h.log, "auth required", http.StatusUnauthorized, nil)
		return
	}

	var req models.ExportFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleResponse(c, h.log, "invalid request", http.StatusBadRequest, err.Error())
		return
	}
	// Export can take a few seconds — give it a longer budget than
	// our DB-default timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	shared := c.Query("share") == "1" || c.Query("share") == "true"
	res, err := h.services.StorageExport().Export(ctx, uid, fileID, req.Provider, req.Path, shared)
	if err != nil {
		handleResponse(c, h.log, "export failed", http.StatusBadRequest, err.Error())
		return
	}
	handleResponse(c, h.log, "exported", http.StatusOK, res)
}

// --- DELETE /api/storage/:provider ----------------------------------------

// DisconnectStorageProvider drops the stored token. The provider's
// own dashboard still shows our app as authorised — only the user
// can revoke that on the provider side; this just makes us forget.
func (h Handler) DisconnectStorageProvider(c *gin.Context) {
	provider := c.Param("provider")
	uid := c.GetString("user_id")
	if uid == "" {
		handleResponse(c, h.log, "auth required", http.StatusUnauthorized, nil)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	if err := h.services.StorageExport().Disconnect(ctx, uid, provider); err != nil {
		handleResponse(c, h.log, "disconnect failed", http.StatusBadRequest, err.Error())
		return
	}
	handleResponse(c, h.log, "disconnected", http.StatusOK, gin.H{"provider": provider})
}
