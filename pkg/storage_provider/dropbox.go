package storage_provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Dropbox implements Provider for Dropbox via the v2 HTTP API.
// Docs: https://www.dropbox.com/developers/documentation/http/documentation
type Dropbox struct {
	AppKey      string
	AppSecret   string
	RedirectURL string
	HTTP        *http.Client
}

func NewDropbox(appKey, appSecret, redirectURL string) *Dropbox {
	return &Dropbox{
		AppKey: appKey, AppSecret: appSecret, RedirectURL: redirectURL,
		HTTP: &http.Client{Timeout: 60 * time.Second},
	}
}

func (d *Dropbox) Name() string { return "dropbox" }

// AuthURL builds an `/oauth2/authorize` URL with offline access so we
// get a refresh_token back. Without `token_access_type=offline`
// Dropbox only returns a short-lived access token, which is unusable
// for batch exports happening hours later.
func (d *Dropbox) AuthURL(state string) string {
	q := url.Values{}
	q.Set("client_id", d.AppKey)
	q.Set("response_type", "code")
	q.Set("redirect_uri", d.RedirectURL)
	q.Set("state", state)
	q.Set("token_access_type", "offline")
	return "https://www.dropbox.com/oauth2/authorize?" + q.Encode()
}

type dropboxTokenResp struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
	UID          string `json:"uid"`
	AccountID    string `json:"account_id"`
}

func (d *Dropbox) Exchange(ctx context.Context, code string) (Token, error) {
	form := url.Values{}
	form.Set("code", code)
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", d.AppKey)
	form.Set("client_secret", d.AppSecret)
	form.Set("redirect_uri", d.RedirectURL)

	tok, err := d.postTokenEndpoint(ctx, form)
	if err != nil {
		return Token{}, err
	}
	// Resolve the user's email opportunistically — it's the same call
	// every other provider exposes, and the UI shows "Connected as
	// you@example.com".
	tok.AccountEmail = d.fetchAccountEmail(ctx, tok.AccessToken)
	return tok, nil
}

func (d *Dropbox) Refresh(ctx context.Context, refreshToken string) (Token, error) {
	if refreshToken == "" {
		return Token{}, fmt.Errorf("dropbox: refresh: empty refresh_token")
	}
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	form.Set("client_id", d.AppKey)
	form.Set("client_secret", d.AppSecret)
	return d.postTokenEndpoint(ctx, form)
}

func (d *Dropbox) postTokenEndpoint(ctx context.Context, form url.Values) (Token, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.dropboxapi.com/oauth2/token", strings.NewReader(form.Encode()))
	if err != nil {
		return Token{}, fmt.Errorf("dropbox token: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := d.HTTP.Do(req)
	if err != nil {
		return Token{}, fmt.Errorf("dropbox token http: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return Token{}, fmt.Errorf("dropbox token status=%d body=%s", resp.StatusCode, string(body))
	}
	var dr dropboxTokenResp
	if err := json.NewDecoder(resp.Body).Decode(&dr); err != nil {
		return Token{}, fmt.Errorf("dropbox token decode: %w", err)
	}
	t := Token{
		AccessToken:  dr.AccessToken,
		RefreshToken: dr.RefreshToken,
		Scope:        dr.Scope,
	}
	if dr.ExpiresIn > 0 {
		t.Expiry = time.Now().Add(time.Duration(dr.ExpiresIn) * time.Second)
	}
	return t, nil
}

type dropboxAccount struct {
	Email string `json:"email"`
}

// fetchAccountEmail hits users/get_current_account once; failure is
// non-fatal because the token still works for uploads.
func (d *Dropbox) fetchAccountEmail(ctx context.Context, accessToken string) string {
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.dropboxapi.com/2/users/get_current_account", strings.NewReader("null"))
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := d.HTTP.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			resp.Body.Close()
		}
		return ""
	}
	defer resp.Body.Close()
	var a dropboxAccount
	if err := json.NewDecoder(resp.Body).Decode(&a); err != nil {
		return ""
	}
	return a.Email
}

// Upload streams the file via files/upload. For >150MB Dropbox needs
// the chunked upload session API — we'd hit that for OCR'd archives
// or merged docs, but the post-conversion outputs we expose are
// generally under that ceiling. Document the limit so a future caller
// knows to swap in the session API if it ever fires.
const dropboxSingleShotLimit = 150 * 1024 * 1024

func (d *Dropbox) Upload(ctx context.Context, tok Token, remotePath, contentType string,
	data io.Reader, size int64, shared bool) (UploadResult, error) {
	if size > dropboxSingleShotLimit {
		return UploadResult{}, fmt.Errorf("dropbox: file too large (%d > 150 MB); chunked upload not yet implemented", size)
	}
	if !strings.HasPrefix(remotePath, "/") {
		remotePath = "/" + remotePath
	}
	// Dropbox-API-Arg header carries the metadata as a JSON blob.
	args, _ := json.Marshal(struct {
		Path           string `json:"path"`
		Mode           string `json:"mode"`
		Autorename     bool   `json:"autorename"`
		Mute           bool   `json:"mute"`
	}{
		Path: remotePath, Mode: "add", Autorename: true, Mute: true,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://content.dropboxapi.com/2/files/upload", data)
	if err != nil {
		return UploadResult{}, fmt.Errorf("dropbox upload: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Dropbox-API-Arg", string(args))
	req.ContentLength = size

	resp, err := d.HTTP.Do(req)
	if err != nil {
		return UploadResult{}, fmt.Errorf("dropbox upload http: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return UploadResult{}, fmt.Errorf("dropbox upload status=%d body=%s", resp.StatusCode, string(body))
	}
	var meta struct {
		PathDisplay string `json:"path_display"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&meta)
	result := UploadResult{RemotePath: meta.PathDisplay}

	if shared {
		result.SharedURL = d.maybeShareLink(ctx, tok.AccessToken, meta.PathDisplay)
	}
	return result, nil
}

// maybeShareLink asks Dropbox for a public preview link. Returns empty
// string on any failure — the upload succeeded regardless, and we
// don't want to fail the whole export just because sharing was
// disabled by an admin.
func (d *Dropbox) maybeShareLink(ctx context.Context, accessToken, path string) string {
	body, _ := json.Marshal(struct {
		Path     string `json:"path"`
		Settings struct {
			RequestedVisibility string `json:"requested_visibility"`
		} `json:"settings"`
	}{Path: path, Settings: struct {
		RequestedVisibility string `json:"requested_visibility"`
	}{RequestedVisibility: "public"}})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.dropboxapi.com/2/sharing/create_shared_link_with_settings",
		strings.NewReader(string(body)))
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := d.HTTP.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			resp.Body.Close()
		}
		return ""
	}
	defer resp.Body.Close()
	var r struct {
		URL string `json:"url"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&r)
	return r.URL
}
