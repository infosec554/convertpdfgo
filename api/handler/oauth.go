package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/pkg/notify"
	"convertpdfgo/pkg/timeouts"
)

const oauthStateCookie = "oauth_state"

// randomState returns 16 random bytes hex-encoded — protects callbacks
// against CSRF.
func randomState() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// rand.Read on Linux only fails if the kernel is broken.
		return "fallback_state"
	}
	return hex.EncodeToString(b)
}

// setOAuthStateCookie writes the state cookie that the provider's redirect
// will have to echo back. Must be SameSite=Lax (not Strict): Strict refuses
// to send the cookie on top-level cross-site navigations, which is exactly
// what google.com → convertpdfgo.com is, so Strict bricked the callback
// with a "invalid oauth state" 400 on every login attempt. Lax keeps the
// cookie safe from CSRF on POST-style cross-site requests while still
// shipping it on the OAuth GET redirect, which is the standard pattern.
func (h Handler) setOAuthStateCookie(c *gin.Context, state string, maxAge int) {
	secure := c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https"
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     oauthStateCookie,
		Value:    state,
		Path:     "/auth",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// GoogleLogin godoc
// @Router       /auth/google [get]
// @Summary      Start Google OAuth flow
// @Tags         auth
// @Success      307
func (h Handler) GoogleLogin(c *gin.Context) {
	state := randomState()
	h.setOAuthStateCookie(c, state, 600)
	c.Redirect(http.StatusTemporaryRedirect, h.googleOAuthConfig.AuthCodeURL(state))
}

// GoogleCallback godoc
// @Router       /auth/google/callback [get]
// @Summary      Google OAuth callback
// @Tags         auth
// @Param        code  query string true "code"
// @Param        state query string true "state"
// @Success      200 {object} models.AuthTokens
func (h Handler) GoogleCallback(c *gin.Context) {
	if !h.checkOAuthState(c) {
		handleResponse(c, h.log, "invalid oauth state", http.StatusBadRequest, nil)
		return
	}
	code := c.Query("code")
	if code == "" {
		handleResponse(c, h.log, "code missing", http.StatusBadRequest, nil)
		return
	}

	tok, err := h.googleOAuthConfig.Exchange(c.Request.Context(), code)
	if err != nil {
		handleResponse(c, h.log, "token exchange failed", http.StatusInternalServerError, err.Error())
		return
	}
	client := h.googleOAuthConfig.Client(c.Request.Context(), tok)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		handleResponse(c, h.log, "fetch user info failed", http.StatusInternalServerError, err.Error())
		return
	}
	defer resp.Body.Close()

	var info struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		handleResponse(c, h.log, "decode user info failed", http.StatusInternalServerError, err.Error())
		return
	}
	if info.Email == "" {
		handleResponse(c, h.log, "google did not return an email", http.StatusBadRequest, nil)
		return
	}
	h.completeOAuthLogin(c, info.Email, info.Name, "google")
}

// GithubLogin godoc
// @Router       /auth/github [get]
// @Summary      Start GitHub OAuth flow
// @Tags         auth
func (h Handler) GithubLogin(c *gin.Context) {
	state := randomState()
	h.setOAuthStateCookie(c, state, 600)
	c.Redirect(http.StatusTemporaryRedirect, h.githubOAuthConfig.AuthCodeURL(state))
}

// GithubCallback godoc
// @Router       /auth/github/callback [get]
// @Summary      GitHub OAuth callback
// @Tags         auth
func (h Handler) GithubCallback(c *gin.Context) {
	if !h.checkOAuthState(c) {
		handleResponse(c, h.log, "invalid oauth state", http.StatusBadRequest, nil)
		return
	}
	code := c.Query("code")
	if code == "" {
		handleResponse(c, h.log, "code missing", http.StatusBadRequest, nil)
		return
	}

	tok, err := h.githubOAuthConfig.Exchange(c.Request.Context(), code)
	if err != nil {
		handleResponse(c, h.log, "token exchange failed", http.StatusInternalServerError, err.Error())
		return
	}
	client := h.githubOAuthConfig.Client(c.Request.Context(), tok)
	resp, err := client.Get("https://api.github.com/user/emails")
	if err != nil {
		handleResponse(c, h.log, "fetch user info failed", http.StatusInternalServerError, err.Error())
		return
	}
	defer resp.Body.Close()

	var emails []struct {
		Email   string `json:"email"`
		Primary bool   `json:"primary"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		handleResponse(c, h.log, "decode user info failed", http.StatusInternalServerError, err.Error())
		return
	}
	email := ""
	for _, e := range emails {
		if e.Primary {
			email = e.Email
			break
		}
	}
	if email == "" && len(emails) > 0 {
		email = emails[0].Email
	}
	if email == "" {
		handleResponse(c, h.log, "github did not return an email", http.StatusBadRequest, nil)
		return
	}
	h.completeOAuthLogin(c, email, "", "github")
}

func (h Handler) checkOAuthState(c *gin.Context) bool {
	cookieState, err := c.Cookie(oauthStateCookie)
	if err != nil || cookieState == "" {
		return false
	}
	queryState := c.Query("state")
	// Clear the cookie regardless of outcome (mirror flags so the browser
	// actually overwrites the original cookie).
	h.setOAuthStateCookie(c, "", -1)
	return cookieState == queryState
}

func (h Handler) completeOAuthLogin(c *gin.Context, email, name, provider string) {
	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	user, err := h.services.User().GetForLoginByEmail(ctx, email)
	userID := user.ID
	role := user.Role

	if errors.Is(err, pgx.ErrNoRows) || userID == "" {
		newID, createErr := h.services.User().Create(ctx, models.CreateUser{
			Name:     name,
			Email:    email,
			Provider: provider,
		})
		if createErr != nil {
			handleResponse(c, h.log, "user creation failed", http.StatusInternalServerError, createErr.Error())
			return
		}
		userID = newID
		role = "user"
		h.log.Info("oauth: created user", logger.String("provider", provider), logger.String("email", email))
		// Surface new sign-ups to admins. Best-effort: a notify failure
		// can't roll back the user we just persisted, so we don't return.
		h.publishAdminEvent(ctx, notify.TypeUserRegistered, notify.UserRegistered{
			UserID: newID,
			Email:  email,
		})
	} else if err != nil {
		handleResponse(c, h.log, "lookup failed", http.StatusInternalServerError, err.Error())
		return
	} else if user.Status != "" && user.Status != "active" {
		handleResponse(c, h.log, "account not active", http.StatusForbidden, nil)
		return
	}
	if role == "" {
		role = "user"
	}

	tokens, err := issueTokens(userID, role)
	if err != nil {
		handleResponse(c, h.log, "token error", http.StatusInternalServerError, err.Error())
		return
	}
	h.audit(c, models.AuthEventOAuthLogin, userID, email, provider)
	h.log.Info("oauth: login ok", logger.String("provider", provider), logger.String("user_id", userID))

	// FRONTEND_URL set ⇒ hand off to the SPA via a 302 to
	// `${FRONTEND_URL}/auth/callback?access_token=…&refresh_token=…`.
	// Query string lets the frontend read tokens with the framework's
	// built-in URL helpers (Next's useSearchParams, Vue Router query,
	// etc.) without an extra window.location.hash branch. Trade-off
	// is that tokens land in edge / access logs as the redirect is
	// served — acceptable for this product, but worth revisiting if
	// we ever ship a stricter security posture.
	// FRONTEND_URL empty ⇒ fall back to JSON (handy for curl/Swagger).
	if h.cfg.FrontendURL != "" {
		target := h.cfg.FrontendURL + "/auth/callback?" +
			"access_token=" + url.QueryEscape(tokens.AccessToken) +
			"&refresh_token=" + url.QueryEscape(tokens.RefreshToken)
		c.Redirect(http.StatusFound, target)
		return
	}
	handleResponse(c, h.log, "login ok", http.StatusOK, tokens)
}
