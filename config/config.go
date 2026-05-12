package config

import (
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/cast"
)

type Config struct {
	PostgresHost     string
	PostgresPort     string
	PostgresUser     string
	PostgresPassword string
	PostgresDB       string

	ServiceName string
	LoggerLevel string

	RedisHost     string
	RedisPort     string
	RedisPassword string
	RedisTTL      time.Duration // ✅ YANGI QO‘SHILDI

	JWTSecretKey string // ✅ YANGI QO'SHILDI
	BrevoAPIKey  string
	BrevoSender  string
	GotenbergURL string

	// OAuth
	AppURL             string
	GoogleClientID     string
	GoogleClientSecret string
	GithubClientID     string
	GithubClientSecret string

	// MinIO / S3 — leave Endpoint empty to fall back to local-disk storage.
	MinIOEndpoint  string
	MinIOAccessKey string
	MinIOSecretKey string
	MinIOBucket    string
	MinIOUseSSL    bool

	// Cloud-storage export targets ("Save to..." in the UI).
	//
	// Each provider is independent: leave its credentials blank to hide
	// the corresponding option from the frontend. Per-user OAuth tokens
	// are stored in user_storage_tokens (added in migration 0013).
	//
	//	Dropbox  — register at https://www.dropbox.com/developers/apps
	//	           (Scoped access app, files.content.write scope).
	//	GDrive   — Google Cloud Console → APIs & Services → Drive API.
	//	           Reuse the existing GOOGLE_CLIENT_ID/SECRET *only* if
	//	           you've added the drive.file scope to the OAuth consent
	//	           screen; otherwise use a dedicated client.
	//	OneDrive — Azure portal → App registrations → Microsoft Graph,
	//	           Files.ReadWrite delegated permission.
	//
	// RedirectURL fields default to AppURL + the provider's callback
	// path when left blank — see DropboxConfigured() etc. below.
	DropboxAppKey      string
	DropboxAppSecret   string
	DropboxRedirectURL string

	GDriveClientID     string
	GDriveClientSecret string
	GDriveRedirectURL  string

	OneDriveClientID     string
	OneDriveClientSecret string
	OneDriveRedirectURL  string
}

// --- export-target capability flags --------------------------------------

// DropboxConfigured reports whether the API can offer "Save to Dropbox"
// — both halves of the app credential must be present. The frontend
// reads this via /api/storage/providers so the UI hides disabled
// targets instead of showing them and failing on click.
func (c Config) DropboxConfigured() bool {
	return c.DropboxAppKey != "" && c.DropboxAppSecret != ""
}

func (c Config) GDriveConfigured() bool {
	return c.GDriveClientID != "" && c.GDriveClientSecret != ""
}

func (c Config) OneDriveConfigured() bool {
	return c.OneDriveClientID != "" && c.OneDriveClientSecret != ""
}

// redirectOrDefault picks the configured RedirectURL or falls back to
// AppURL + path. Centralised so every provider follows the same rule.
func (c Config) redirectOrDefault(explicit, callbackPath string) string {
	if explicit != "" {
		return explicit
	}
	return c.AppURL + callbackPath
}

// DropboxRedirect returns the Dropbox OAuth callback URL the API
// should advertise. Uses the explicit env value if set; otherwise
// derives one from AppURL.
func (c Config) DropboxRedirect() string {
	return c.redirectOrDefault(c.DropboxRedirectURL, "/oauth/dropbox/callback")
}

func (c Config) GDriveRedirect() string {
	return c.redirectOrDefault(c.GDriveRedirectURL, "/oauth/gdrive/callback")
}

func (c Config) OneDriveRedirect() string {
	return c.redirectOrDefault(c.OneDriveRedirectURL, "/oauth/onedrive/callback")
}

func Load() Config {
	if err := godotenv.Load(); err != nil {
		fmt.Println("error!!!", err)
	}

	cfg := Config{}

	cfg.PostgresHost = cast.ToString(getOrReturnDefault("POSTGRES_HOST", "localhost"))
	cfg.PostgresPort = cast.ToString(getOrReturnDefault("POSTGRES_PORT", "5432"))
	cfg.PostgresUser = cast.ToString(getOrReturnDefault("POSTGRES_USER", "postgres"))
	cfg.PostgresPassword = cast.ToString(getOrReturnDefault("POSTGRES_PASSWORD", "1234"))
	cfg.PostgresDB = cast.ToString(getOrReturnDefault("POSTGRES_DB", "authservice"))

	cfg.ServiceName = cast.ToString(getOrReturnDefault("SERVICE_NAME", "convertpdfgo"))
	cfg.LoggerLevel = cast.ToString(getOrReturnDefault("LOGGER_LEVEL", "debug"))

	cfg.JWTSecretKey = cast.ToString(getOrReturnDefault("JWT_SECRET_KEY", "default_secret_key"))

	cfg.RedisHost = cast.ToString(getOrReturnDefault("REDIS_HOST", "localhost"))
	cfg.RedisPort = cast.ToString(getOrReturnDefault("REDIS_PORT", "6379"))
	cfg.RedisPassword = cast.ToString(getOrReturnDefault("REDIS_PASSWORD", "1234"))
	cfg.RedisTTL = cast.ToDuration(getOrReturnDefault("REDIS_TTL", "5m"))

	// 🔧 Brevo API configlar:
	cfg.BrevoAPIKey = cast.ToString(getOrReturnDefault("BREVO_API_KEY", ""))
	cfg.BrevoSender = cast.ToString(getOrReturnDefault("BREVO_SENDER", "noreply@convertpdfgo.com"))
	cfg.GotenbergURL = cast.ToString(getOrReturnDefault("GOTENBERG_URL", "http://localhost:3000"))

	// OAuth configlar:
	cfg.AppURL = cast.ToString(getOrReturnDefault("APP_URL", "http://localhost:8080"))
	cfg.GoogleClientID = cast.ToString(getOrReturnDefault("GOOGLE_CLIENT_ID", ""))
	cfg.GoogleClientSecret = cast.ToString(getOrReturnDefault("GOOGLE_CLIENT_SECRET", ""))
	cfg.GithubClientID = cast.ToString(getOrReturnDefault("GITHUB_CLIENT_ID", ""))
	cfg.GithubClientSecret = cast.ToString(getOrReturnDefault("GITHUB_CLIENT_SECRET", ""))

	// MinIO (optional — empty endpoint = local disk).
	cfg.MinIOEndpoint = cast.ToString(getOrReturnDefault("MINIO_ENDPOINT", ""))
	cfg.MinIOAccessKey = cast.ToString(getOrReturnDefault("MINIO_ACCESS_KEY", "minioadmin"))
	cfg.MinIOSecretKey = cast.ToString(getOrReturnDefault("MINIO_SECRET_KEY", "minioadmin"))
	cfg.MinIOBucket = cast.ToString(getOrReturnDefault("MINIO_BUCKET", "convertpdfgo"))
	cfg.MinIOUseSSL = cast.ToBool(getOrReturnDefault("MINIO_USE_SSL", false))

	// Cloud-export targets (optional — each empty by default).
	cfg.DropboxAppKey = cast.ToString(getOrReturnDefault("DROPBOX_APP_KEY", ""))
	cfg.DropboxAppSecret = cast.ToString(getOrReturnDefault("DROPBOX_APP_SECRET", ""))
	cfg.DropboxRedirectURL = cast.ToString(getOrReturnDefault("DROPBOX_REDIRECT_URL", ""))

	cfg.GDriveClientID = cast.ToString(getOrReturnDefault("GDRIVE_CLIENT_ID", ""))
	cfg.GDriveClientSecret = cast.ToString(getOrReturnDefault("GDRIVE_CLIENT_SECRET", ""))
	cfg.GDriveRedirectURL = cast.ToString(getOrReturnDefault("GDRIVE_REDIRECT_URL", ""))

	cfg.OneDriveClientID = cast.ToString(getOrReturnDefault("ONEDRIVE_CLIENT_ID", ""))
	cfg.OneDriveClientSecret = cast.ToString(getOrReturnDefault("ONEDRIVE_CLIENT_SECRET", ""))
	cfg.OneDriveRedirectURL = cast.ToString(getOrReturnDefault("ONEDRIVE_REDIRECT_URL", ""))

	return cfg
}

func getOrReturnDefault(key string, defaultValue interface{}) interface{} {
	value := os.Getenv(key)
	if value != "" {
		return value
	}

	return defaultValue
}
