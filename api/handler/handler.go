package handler

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"

	"convertpdfgo/api/models"
	"convertpdfgo/config"
	"convertpdfgo/pkg/blacklist"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/pkg/loginlimiter"
	"convertpdfgo/service"
	"convertpdfgo/storage"
)

type Handler struct {
	services          service.IServiceManager
	log               logger.ILogger
	cfg               config.Config
	queue             *asynq.Client
	bl                *blacklist.Blacklist
	loginLimit        *loginlimiter.Limiter
	googleOAuthConfig *oauth2.Config
	githubOAuthConfig *oauth2.Config
	// redis carries short-lived flow state (OAuth CSRF tokens,
	// idempotency keys) where introducing a typed wrapper would be
	// more ceremony than the use case warrants.
	redis storage.IRedisStorage
}

func New(services service.IServiceManager, log logger.ILogger, cfg config.Config, queue *asynq.Client, redis storage.IRedisStorage) Handler {
	return Handler{
		services:   services,
		log:        log,
		cfg:        cfg,
		queue:      queue,
		bl:         blacklist.New(redis),
		loginLimit: loginlimiter.New(redis, 5, 10*time.Minute),
		redis:      redis,
		googleOAuthConfig: &oauth2.Config{
			ClientID:     cfg.GoogleClientID,
			ClientSecret: cfg.GoogleClientSecret,
			RedirectURL:  cfg.AppURL + "/auth/google/callback",
			Scopes:       []string{"email", "profile"},
			Endpoint:     google.Endpoint,
		},
		githubOAuthConfig: &oauth2.Config{
			ClientID:     cfg.GithubClientID,
			ClientSecret: cfg.GithubClientSecret,
			RedirectURL:  cfg.AppURL + "/auth/github/callback",
			Scopes:       []string{"user:email"},
			Endpoint:     github.Endpoint,
		},
	}
}

// userPtr collapses the JWT middleware's user_id context value into the
// *string shape every service Create takes: nil for guests, pointer to
// the UUID string for authenticated users. Replaces the ad-hoc
//
//	var uid *string
//	if v := c.GetString("user_id"); v != "" { uid = &v }
//
// dance that was copy-pasted across 25 handlers in slightly different
// forms (string vs *string, &uid vs &v).
func userPtr(c *gin.Context) *string {
	if uid := c.GetString("user_id"); uid != "" {
		return &uid
	}
	return nil
}

func handleResponse(c *gin.Context, log logger.ILogger, msg string, statusCode int, data interface{}) {
	resp := models.Response{}

	switch code := statusCode; {
	case code < 400:
		resp.Description = "OK"
		log.Info("OK", logger.String("msg", msg), logger.Any("status", code))
	case code == 401:
		resp.Description = "Unauthorized"
	case code < 500:
		resp.Description = "Bad Request"
		log.Error("BAD REQUEST", logger.String("msg", msg), logger.Any("status", code))
	default:
		resp.Description = "Internal Server Error"
		log.Error("INTERNAL SERVER ERROR", logger.String("msg", msg), logger.Any("status", code), logger.Any("detail", data))
		// Never echo raw err.Error() back on 5xx — it can leak DB/file paths.
		// Replace with a generic, request-id-tagged hint.
		if rid, ok := c.Get("request_id"); ok {
			data = gin.H{"request_id": rid}
		} else {
			data = nil
		}
	}

	resp.StatusCode = statusCode
	resp.Data = data

	c.JSON(resp.StatusCode, resp)
}
