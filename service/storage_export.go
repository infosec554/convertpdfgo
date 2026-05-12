// "Save to ..." service. Wraps:
//
//   - the per-provider OAuth dance (start → consent → exchange → store)
//   - the export call itself (fetch our file from disk, hand it to the
//     provider's Upload method with a refreshed token)
//   - the Providers() list that the frontend uses to render the
//     dropdown
//
// pkg/storage_provider keeps the wire-protocol details; this file
// is the orchestration around them — it owns the token table, the
// CSRF state, and the auto-refresh-when-expired check.
package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"convertpdfgo/api/models"
	"convertpdfgo/config"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/pkg/storage_provider"
	"convertpdfgo/storage"
)

// StorageExportService is the public surface the HTTP layer talks to.
type StorageExportService interface {
	// Providers lists every export target with capability + connection
	// status for the calling user. Empty userID → only Configured is
	// trustworthy (Connected always false).
	Providers(ctx context.Context, userID string) []models.StorageProviderInfo

	// AuthURL builds the OAuth redirect URL for the given provider and
	// returns the state token the callback must echo back.
	AuthURL(provider string) (authURL, state string, err error)

	// FinishOAuth handles the callback: exchanges the code, persists
	// the token row. Returns the provider name on success so the
	// handler can redirect the browser somewhere useful.
	FinishOAuth(ctx context.Context, userID, provider, code string) (string, error)

	// Export uploads our local file (file_id) to the user's account on
	// the given provider. Refreshes the access_token if needed.
	Export(ctx context.Context, userID, fileID, provider, remotePath string, shared bool) (models.ExportFileResponse, error)

	// Disconnect deletes the stored token for (user, provider) — the
	// user explicitly chose to revoke our access.
	Disconnect(ctx context.Context, userID, provider string) error
}

type storageExportService struct {
	stg      storage.IStorageTokenStorage
	files    storage.IFileStorage
	log      logger.ILogger
	cfg      config.Config
	dropbox  storage_provider.Provider
	gdrive   storage_provider.Provider
	onedrive storage_provider.Provider
}

func NewStorageExportService(stg storage.IStorage, log logger.ILogger, cfg config.Config) StorageExportService {
	s := &storageExportService{
		stg:   stg.StorageToken(),
		files: stg.File(),
		log:   log,
		cfg:   cfg,
	}
	// Each provider is instantiated only when its credentials are
	// present. Providers()'s "Configured" flag reads off these
	// non-nil checks.
	if cfg.DropboxConfigured() {
		s.dropbox = storage_provider.NewDropbox(cfg.DropboxAppKey, cfg.DropboxAppSecret, cfg.DropboxRedirect())
	}
	if cfg.GDriveConfigured() {
		s.gdrive = storage_provider.NewGDrive(cfg.GDriveClientID, cfg.GDriveClientSecret, cfg.GDriveRedirect())
	}
	if cfg.OneDriveConfigured() {
		s.onedrive = storage_provider.NewOneDrive(cfg.OneDriveClientID, cfg.OneDriveClientSecret, cfg.OneDriveRedirect())
	}
	return s
}

func (s *storageExportService) Providers(ctx context.Context, userID string) []models.StorageProviderInfo {
	connected := map[string]models.UserStorageToken{}
	if userID != "" {
		if rows, err := s.stg.ListByUser(ctx, userID); err == nil {
			for _, r := range rows {
				connected[r.Provider] = r
			}
		}
	}
	mk := func(name, display string, configured bool) models.StorageProviderInfo {
		info := models.StorageProviderInfo{
			Name: name, DisplayName: display, Configured: configured,
		}
		if t, ok := connected[name]; ok {
			info.Connected = true
			info.AccountEmail = t.AccountEmail
		}
		return info
	}
	return []models.StorageProviderInfo{
		mk(models.ProviderDropbox, "Dropbox", s.dropbox != nil),
		mk(models.ProviderGDrive, "Google Drive", s.gdrive != nil),
		mk(models.ProviderOneDrive, "OneDrive", s.onedrive != nil),
	}
}

func (s *storageExportService) AuthURL(provider string) (string, string, error) {
	p, err := s.lookup(provider)
	if err != nil {
		return "", "", err
	}
	state, err := newStateToken()
	if err != nil {
		return "", "", err
	}
	return p.AuthURL(state), state, nil
}

func (s *storageExportService) FinishOAuth(ctx context.Context, userID, provider, code string) (string, error) {
	p, err := s.lookup(provider)
	if err != nil {
		return "", err
	}
	tok, err := p.Exchange(ctx, code)
	if err != nil {
		return "", fmt.Errorf("exchange: %w", err)
	}
	if err := s.persistToken(ctx, userID, provider, tok); err != nil {
		return "", err
	}
	return provider, nil
}

func (s *storageExportService) Export(ctx context.Context, userID, fileID, provider, remotePath string, shared bool) (models.ExportFileResponse, error) {
	p, err := s.lookup(provider)
	if err != nil {
		return models.ExportFileResponse{}, err
	}
	row, err := s.stg.Get(ctx, userID, provider)
	if err != nil {
		return models.ExportFileResponse{}, fmt.Errorf("token: %w (try connecting %s first)", err, provider)
	}

	// Auto-refresh if the stored token is past its expiry. Providers
	// without an expiry (some Dropbox scopes) skip this branch.
	tok := storage_provider.Token{
		AccessToken:  row.AccessToken,
		RefreshToken: row.RefreshToken,
	}
	if row.ExpiresAt != nil {
		tok.Expiry = *row.ExpiresAt
	}
	if tok.Expired() {
		newTok, rerr := p.Refresh(ctx, row.RefreshToken)
		if rerr != nil {
			return models.ExportFileResponse{}, fmt.Errorf("refresh: %w", rerr)
		}
		tok.AccessToken = newTok.AccessToken
		if newTok.RefreshToken != "" {
			tok.RefreshToken = newTok.RefreshToken
		}
		if !newTok.Expiry.IsZero() {
			tok.Expiry = newTok.Expiry
		}
		// Persist the new access_token in the cheapest update path.
		var expiry *time.Time
		if !newTok.Expiry.IsZero() {
			e := newTok.Expiry
			expiry = &e
		}
		if uerr := s.stg.UpdateAccessOnly(ctx, userID, provider, newTok.AccessToken, expiry); uerr != nil {
			s.log.Warning("storage_export: persist refreshed token", logger.Error(uerr))
		}
	}

	// Look up the file we're about to ship.
	f, err := s.files.GetByID(ctx, fileID)
	if err != nil {
		return models.ExportFileResponse{}, fmt.Errorf("file: %w", err)
	}
	src, err := os.Open(f.FilePath)
	if err != nil {
		return models.ExportFileResponse{}, fmt.Errorf("open: %w", err)
	}
	defer src.Close()

	if remotePath == "" {
		remotePath = filepath.Base(f.FilePath)
	}
	contentType := f.FileType
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	res, err := p.Upload(ctx, tok, remotePath, contentType, src, f.FileSize, shared)
	if err != nil {
		return models.ExportFileResponse{}, fmt.Errorf("upload: %w", err)
	}
	return models.ExportFileResponse{
		Provider:   provider,
		RemotePath: res.RemotePath,
		SharedURL:  res.SharedURL,
	}, nil
}

func (s *storageExportService) Disconnect(ctx context.Context, userID, provider string) error {
	if _, err := s.lookup(provider); err != nil {
		return err
	}
	return s.stg.Delete(ctx, userID, provider)
}

// --- internals -----------------------------------------------------------

func (s *storageExportService) lookup(provider string) (storage_provider.Provider, error) {
	switch provider {
	case models.ProviderDropbox:
		if s.dropbox == nil {
			return nil, fmt.Errorf("provider not configured: dropbox")
		}
		return s.dropbox, nil
	case models.ProviderGDrive:
		if s.gdrive == nil {
			return nil, fmt.Errorf("provider not configured: gdrive")
		}
		return s.gdrive, nil
	case models.ProviderOneDrive:
		if s.onedrive == nil {
			return nil, fmt.Errorf("provider not configured: onedrive")
		}
		return s.onedrive, nil
	}
	return nil, fmt.Errorf("unknown provider: %s", provider)
}

func (s *storageExportService) persistToken(ctx context.Context, userID, provider string, tok storage_provider.Token) error {
	row := &models.UserStorageToken{
		UserID: userID, Provider: provider,
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		Scope:        tok.Scope,
		AccountEmail: tok.AccountEmail,
	}
	if !tok.Expiry.IsZero() {
		e := tok.Expiry
		row.ExpiresAt = &e
	}
	return s.stg.Upsert(ctx, row)
}

// newStateToken returns 32 hex chars of random — enough entropy to
// make the OAuth state parameter unguessable.
func newStateToken() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
