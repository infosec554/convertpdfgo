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

	return cfg
}

func getOrReturnDefault(key string, defaultValue interface{}) interface{} {
	value := os.Getenv(key)
	if value != "" {
		return value
	}

	return defaultValue
}
