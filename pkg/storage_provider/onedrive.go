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

// OneDrive implements Provider for Microsoft Graph (consumer + work
// accounts both work). Docs:
// https://learn.microsoft.com/graph/api/driveitem-put-content
//
// Tenant: we use "common" so both personal and AAD-managed accounts
// can connect. If you want to restrict to your tenant, swap "common"
// for the tenant ID.
type OneDrive struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	HTTP         *http.Client
}

func NewOneDrive(clientID, clientSecret, redirectURL string) *OneDrive {
	return &OneDrive{
		ClientID: clientID, ClientSecret: clientSecret, RedirectURL: redirectURL,
		HTTP: &http.Client{Timeout: 60 * time.Second},
	}
}

func (o *OneDrive) Name() string { return "onedrive" }

// AuthURL builds the Microsoft identity-platform v2 authorize URL.
// `offline_access` is what makes the refresh_token appear in the
// token response; `Files.ReadWrite` lets us write into the user's
// OneDrive (any folder we create).
func (o *OneDrive) AuthURL(state string) string {
	q := url.Values{}
	q.Set("client_id", o.ClientID)
	q.Set("response_type", "code")
	q.Set("redirect_uri", o.RedirectURL)
	q.Set("state", state)
	q.Set("scope", "offline_access Files.ReadWrite User.Read")
	q.Set("response_mode", "query")
	return "https://login.microsoftonline.com/common/oauth2/v2.0/authorize?" + q.Encode()
}

type msTokenResp struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
	TokenType    string `json:"token_type"`
}

func (o *OneDrive) Exchange(ctx context.Context, code string) (Token, error) {
	form := url.Values{}
	form.Set("code", code)
	form.Set("client_id", o.ClientID)
	form.Set("client_secret", o.ClientSecret)
	form.Set("redirect_uri", o.RedirectURL)
	form.Set("grant_type", "authorization_code")
	tok, err := o.postTokenEndpoint(ctx, form)
	if err != nil {
		return Token{}, err
	}
	tok.AccountEmail = o.fetchUserEmail(ctx, tok.AccessToken)
	return tok, nil
}

func (o *OneDrive) Refresh(ctx context.Context, refreshToken string) (Token, error) {
	if refreshToken == "" {
		return Token{}, fmt.Errorf("onedrive: refresh: empty refresh_token")
	}
	form := url.Values{}
	form.Set("refresh_token", refreshToken)
	form.Set("client_id", o.ClientID)
	form.Set("client_secret", o.ClientSecret)
	form.Set("grant_type", "refresh_token")
	t, err := o.postTokenEndpoint(ctx, form)
	if err != nil {
		return Token{}, err
	}
	if t.RefreshToken == "" {
		t.RefreshToken = refreshToken
	}
	return t, nil
}

func (o *OneDrive) postTokenEndpoint(ctx context.Context, form url.Values) (Token, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://login.microsoftonline.com/common/oauth2/v2.0/token",
		strings.NewReader(form.Encode()))
	if err != nil {
		return Token{}, fmt.Errorf("onedrive token: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := o.HTTP.Do(req)
	if err != nil {
		return Token{}, fmt.Errorf("onedrive token http: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return Token{}, fmt.Errorf("onedrive token status=%d body=%s", resp.StatusCode, string(body))
	}
	var mr msTokenResp
	if err := json.NewDecoder(resp.Body).Decode(&mr); err != nil {
		return Token{}, fmt.Errorf("onedrive token decode: %w", err)
	}
	t := Token{
		AccessToken:  mr.AccessToken,
		RefreshToken: mr.RefreshToken,
		Scope:        mr.Scope,
	}
	if mr.ExpiresIn > 0 {
		t.Expiry = time.Now().Add(time.Duration(mr.ExpiresIn) * time.Second)
	}
	return t, nil
}

func (o *OneDrive) fetchUserEmail(ctx context.Context, accessToken string) string {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://graph.microsoft.com/v1.0/me", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := o.HTTP.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			resp.Body.Close()
		}
		return ""
	}
	defer resp.Body.Close()
	var u struct {
		Mail              string `json:"mail"`
		UserPrincipalName string `json:"userPrincipalName"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&u)
	if u.Mail != "" {
		return u.Mail
	}
	return u.UserPrincipalName
}

// Upload uses /drives/me/root:/<path>:/content for simple uploads
// (up to 4 MB per Microsoft's docs). Larger files need a separate
// `createUploadSession` flow — same caveat as Dropbox; document and
// bail loudly so callers don't get a confusing 400 from Graph.
const oneDriveSimpleUploadLimit = 4 * 1024 * 1024

func (o *OneDrive) Upload(ctx context.Context, tok Token, remotePath, contentType string,
	data io.Reader, size int64, shared bool) (UploadResult, error) {
	if size > oneDriveSimpleUploadLimit {
		return UploadResult{}, fmt.Errorf("onedrive: file too large (%d > 4 MB); upload session not yet implemented", size)
	}
	remotePath = strings.TrimPrefix(remotePath, "/")
	if remotePath == "" {
		remotePath = "convertpdfgo-export.pdf"
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	endpoint := "https://graph.microsoft.com/v1.0/me/drive/root:/" +
		url.PathEscape(remotePath) + ":/content"

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, data)
	if err != nil {
		return UploadResult{}, fmt.Errorf("onedrive upload: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	req.Header.Set("Content-Type", contentType)
	req.ContentLength = size

	resp, err := o.HTTP.Do(req)
	if err != nil {
		return UploadResult{}, fmt.Errorf("onedrive upload http: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return UploadResult{}, fmt.Errorf("onedrive upload status=%d body=%s", resp.StatusCode, string(body))
	}
	var meta struct {
		ID             string `json:"id"`
		Name           string `json:"name"`
		WebURL         string `json:"webUrl"`
		ParentRef      struct {
			Path string `json:"path"`
		} `json:"parentReference"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&meta)
	result := UploadResult{RemotePath: meta.Name}

	if shared && meta.ID != "" {
		if link := o.createShareLink(ctx, tok.AccessToken, meta.ID); link != "" {
			result.SharedURL = link
		} else if meta.WebURL != "" {
			result.SharedURL = meta.WebURL
		}
	}
	return result, nil
}

func (o *OneDrive) createShareLink(ctx context.Context, accessToken, itemID string) string {
	body, _ := json.Marshal(struct {
		Type  string `json:"type"`
		Scope string `json:"scope"`
	}{Type: "view", Scope: "anonymous"})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://graph.microsoft.com/v1.0/me/drive/items/"+itemID+"/createLink",
		strings.NewReader(string(body)))
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := o.HTTP.Do(req)
	if err != nil || resp.StatusCode/100 != 2 {
		if resp != nil {
			resp.Body.Close()
		}
		return ""
	}
	defer resp.Body.Close()
	var r struct {
		Link struct {
			WebURL string `json:"webUrl"`
		} `json:"link"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&r)
	return r.Link.WebURL
}
