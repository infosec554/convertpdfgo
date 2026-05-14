package handler

import (
	"context"
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
	"convertpdfgo/pkg/notify"
	"convertpdfgo/pkg/wshub"
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
	// hub holds live WS connections; nil when realtime is disabled.
	hub *wshub.Hub
	// bus publishes events to the cross-process backplane (Redis pub/sub
	// in prod, Noop when realtime is disabled). The API process both
	// publishes (admin events) and subscribes (job updates from worker).
	bus notify.Bus
}

func New(services service.IServiceManager, log logger.ILogger, cfg config.Config, queue *asynq.Client, redis storage.IRedisStorage, hub *wshub.Hub, bus notify.Bus) Handler {
	if bus == nil {
		bus = notify.Noop{}
	}
	return Handler{
		services:   services,
		log:        log,
		cfg:        cfg,
		queue:      queue,
		bl:         blacklist.New(redis),
		loginLimit: loginlimiter.New(redis, 5, 10*time.Minute),
		redis:      redis,
		hub:        hub,
		bus:        bus,
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

// publishAdminEvent fires a typed admin-broadcast envelope through the bus.
// Best-effort: marshal or transport failures are logged but never returned
// to the caller — notifications are fire-and-forget on every code path.
func (h Handler) publishAdminEvent(ctx context.Context, t notify.Type, data any) {
	if h.bus == nil {
		return
	}
	env, err := notify.NewEnvelope(t, data)
	if err != nil {
		h.log.Warning("notify: build envelope", logger.Error(err), logger.String("type", string(t)))
		return
	}
	if err := h.bus.PublishAdmin(ctx, env); err != nil {
		h.log.Warning("notify: publish admin", logger.Error(err), logger.String("type", string(t)))
	}
}

// publishUserEvent fires a typed envelope at one user's WS subscribers.
// Empty userID is a no-op (guest jobs don't get notifications).
func (h Handler) publishUserEvent(ctx context.Context, userID string, t notify.Type, data any) {
	if h.bus == nil || userID == "" {
		return
	}
	env, err := notify.NewEnvelope(t, data)
	if err != nil {
		h.log.Warning("notify: build envelope", logger.Error(err), logger.String("type", string(t)))
		return
	}
	if err := h.bus.PublishUser(ctx, userID, env); err != nil {
		h.log.Warning("notify: publish user", logger.Error(err), logger.String("type", string(t)))
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
