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

// GDrive implements Provider for Google Drive via the v3 REST API.
// Docs: https://developers.google.com/drive/api/v3/reference
//
// Scope: drive.file lets us read/write only files our app creates —
// far safer than full drive access. Users see the consent screen
// say "create files in your Drive" instead of "see all your files".
type GDrive struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	HTTP         *http.Client
}

func NewGDrive(clientID, clientSecret, redirectURL string) *GDrive {
	return &GDrive{
		ClientID: clientID, ClientSecret: clientSecret, RedirectURL: redirectURL,
		HTTP: &http.Client{Timeout: 60 * time.Second},
	}
}

func (g *GDrive) Name() string { return "gdrive" }

// AuthURL builds Google's OAuth2 v2 URL. `access_type=offline` is
// what produces a refresh_token; `prompt=consent` makes Google
// re-issue the refresh_token even if the user has approved us before
// (otherwise we'd get one only the first time and re-auths would
// silently lose offline access).
func (g *GDrive) AuthURL(state string) string {
	q := url.Values{}
	q.Set("client_id", g.ClientID)
	q.Set("response_type", "code")
	q.Set("redirect_uri", g.RedirectURL)
	q.Set("state", state)
	q.Set("scope", "https://www.googleapis.com/auth/drive.file https://www.googleapis.com/auth/userinfo.email")
	q.Set("access_type", "offline")
	q.Set("prompt", "consent")
	return "https://accounts.google.com/o/oauth2/v2/auth?" + q.Encode()
}

type googleTokenResp struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
	TokenType    string `json:"token_type"`
}

func (g *GDrive) Exchange(ctx context.Context, code string) (Token, error) {
	form := url.Values{}
	form.Set("code", code)
	form.Set("client_id", g.ClientID)
	form.Set("client_secret", g.ClientSecret)
	form.Set("redirect_uri", g.RedirectURL)
	form.Set("grant_type", "authorization_code")
	tok, err := g.postTokenEndpoint(ctx, form)
	if err != nil {
		return Token{}, err
	}
	tok.AccountEmail = g.fetchUserEmail(ctx, tok.AccessToken)
	return tok, nil
}

func (g *GDrive) Refresh(ctx context.Context, refreshToken string) (Token, error) {
	if refreshToken == "" {
		return Token{}, fmt.Errorf("gdrive: refresh: empty refresh_token")
	}
	form := url.Values{}
	form.Set("refresh_token", refreshToken)
	form.Set("client_id", g.ClientID)
	form.Set("client_secret", g.ClientSecret)
	form.Set("grant_type", "refresh_token")
	t, err := g.postTokenEndpoint(ctx, form)
	if err != nil {
		return Token{}, err
	}
	// Google omits refresh_token on refresh calls — keep the old one.
	if t.RefreshToken == "" {
		t.RefreshToken = refreshToken
	}
	return t, nil
}

func (g *GDrive) postTokenEndpoint(ctx context.Context, form url.Values) (Token, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://oauth2.googleapis.com/token", strings.NewReader(form.Encode()))
	if err != nil {
		return Token{}, fmt.Errorf("gdrive token: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := g.HTTP.Do(req)
	if err != nil {
		return Token{}, fmt.Errorf("gdrive token http: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return Token{}, fmt.Errorf("gdrive token status=%d body=%s", resp.StatusCode, string(body))
	}
	var gr googleTokenResp
	if err := json.NewDecoder(resp.Body).Decode(&gr); err != nil {
		return Token{}, fmt.Errorf("gdrive token decode: %w", err)
	}
	t := Token{
		AccessToken:  gr.AccessToken,
		RefreshToken: gr.RefreshToken,
		Scope:        gr.Scope,
	}
	if gr.ExpiresIn > 0 {
		t.Expiry = time.Now().Add(time.Duration(gr.ExpiresIn) * time.Second)
	}
	return t, nil
}

func (g *GDrive) fetchUserEmail(ctx context.Context, accessToken string) string {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://www.googleapis.com/oauth2/v3/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := g.HTTP.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			resp.Body.Close()
		}
		return ""
	}
	defer resp.Body.Close()
	var u struct {
		Email string `json:"email"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&u)
	return u.Email
}

// Upload uses Drive's multipart upload (efficient single-shot for
// files up to a few hundred MB; larger needs resumable upload).
// `remotePath` here means "filename in My Drive" — Drive identifies
// folders by ID, not by path, so deep nesting would require listing /
// creating folders first. Out of scope for this commit; the file
// lands at the root with the requested name.
func (g *GDrive) Upload(ctx context.Context, tok Token, remotePath, contentType string,
	data io.Reader, size int64, shared bool) (UploadResult, error) {
	name := strings.TrimPrefix(remotePath, "/")
	if name == "" {
		name = "convertpdfgo-export.pdf"
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Build the multipart body manually so we can stream the file
	// part instead of buffering the whole thing — keeps memory flat
	// for big OCR'd PDFs.
	body, ct := gdriveMultipartBody(name, contentType, data)
	q := url.Values{}
	q.Set("uploadType", "multipart")
	q.Set("fields", "id,name,webViewLink")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://www.googleapis.com/upload/drive/v3/files?"+q.Encode(), body)
	if err != nil {
		return UploadResult{}, fmt.Errorf("gdrive upload: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	req.Header.Set("Content-Type", ct)

	resp, err := g.HTTP.Do(req)
	if err != nil {
		return UploadResult{}, fmt.Errorf("gdrive upload http: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return UploadResult{}, fmt.Errorf("gdrive upload status=%d body=%s", resp.StatusCode, string(body))
	}
	var meta struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		WebViewLink string `json:"webViewLink"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&meta)
	result := UploadResult{RemotePath: meta.Name}

	if shared && meta.ID != "" {
		if link := g.makeAnyoneReader(ctx, tok.AccessToken, meta.ID); link != "" {
			result.SharedURL = link
		} else if meta.WebViewLink != "" {
			result.SharedURL = meta.WebViewLink // login-required link is better than nothing
		}
	}
	return result, nil
}

// gdriveMultipartBody assembles a `multipart/related` body Drive
// accepts for an uploadType=multipart call.
func gdriveMultipartBody(name, contentType string, data io.Reader) (io.Reader, string) {
	const boundary = "convertpdfgoboundary"
	meta := fmt.Sprintf(`{"name":%q}`, name)
	header := "--" + boundary + "\r\n" +
		"Content-Type: application/json; charset=UTF-8\r\n\r\n" +
		meta + "\r\n" +
		"--" + boundary + "\r\n" +
		"Content-Type: " + contentType + "\r\n\r\n"
	footer := "\r\n--" + boundary + "--\r\n"
	body := io.MultiReader(strings.NewReader(header), data, strings.NewReader(footer))
	return body, "multipart/related; boundary=" + boundary
}

// makeAnyoneReader flips the new file to "anyone with the link" and
// returns the share URL. Failure is non-fatal — caller logs.
func (g *GDrive) makeAnyoneReader(ctx context.Context, accessToken, fileID string) string {
	body, _ := json.Marshal(struct {
		Role string `json:"role"`
		Type string `json:"type"`
	}{Role: "reader", Type: "anyone"})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://www.googleapis.com/drive/v3/files/"+fileID+"/permissions",
		strings.NewReader(string(body)))
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := g.HTTP.Do(req)
	if err != nil || resp.StatusCode/100 != 2 {
		if resp != nil {
			resp.Body.Close()
		}
		return ""
	}
	resp.Body.Close()
	return "https://drive.google.com/file/d/" + fileID + "/view"
}
