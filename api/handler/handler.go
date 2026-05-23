package handler

import (
	"context"
	"strings"
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
		// 4xx payloads commonly carry the raw `err.Error()` string from
		// Gin's validator ("Key: 'XxxRequest.Field' Error:Field
		// validation for 'Field' failed on the 'required' tag\n..."),
		// which is fine for logs but useless to a UI. Reshape it into a
		// map[field]reason so the frontend can drive form-field errors
		// directly. Non-validator bodies pass through untouched.
		data = humaniseValidationError(data)
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

// humaniseValidationError converts the raw Go-validator error string that
// Gin returns from ShouldBindJSON into something a frontend can consume
// directly. The validator emits one line per failing field shaped like:
//
//	Key: 'WatermarkRequest.Mode' Error:Field validation for 'Mode' failed on the 'required' tag
//
// We pull the field name and the failing tag out of each line and map
// the tag to a short, human-readable reason. Anything that doesn't look
// like the validator format (a plain string from a handler, a structured
// gin.H, nil, etc.) is returned unchanged.
func humaniseValidationError(data interface{}) interface{} {
	raw, ok := data.(string)
	if !ok || !strings.HasPrefix(raw, "Key: '") {
		return data
	}
	errs := map[string]string{}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "Key: '") {
			continue
		}
		// Extract field: between "Key: 'Struct." and "'".
		fieldStart := strings.Index(line, ".") + 1
		fieldEnd := strings.Index(line[fieldStart:], "'")
		if fieldStart <= 0 || fieldEnd < 0 {
			continue
		}
		field := line[fieldStart : fieldStart+fieldEnd]
		// Extract tag: between "failed on the '" and "' tag".
		tagStart := strings.LastIndex(line, "failed on the '") + len("failed on the '")
		tagEnd := strings.LastIndex(line, "' tag")
		tag := ""
		if tagStart > len("failed on the '")-1 && tagEnd > tagStart {
			tag = line[tagStart:tagEnd]
		}
		errs[toSnakeCase(field)] = validationReason(tag, field)
	}
	if len(errs) == 0 {
		return data
	}
	return gin.H{"errors": errs}
}

// validationReason maps a validator tag to a short, user-facing message.
// The default fall-through ("is invalid") keeps unknown tags safe rather
// than echoing them verbatim — adding a new tag means writing the message
// here, which forces us to consider how it should render in the UI.
func validationReason(tag, field string) string {
	switch tag {
	case "required":
		return field + " is required"
	case "email":
		return "must be a valid email address"
	case "uuid":
		return "must be a valid UUID"
	case "min":
		return "value is too short"
	case "max":
		return "value is too long"
	case "len":
		return "value has the wrong length"
	case "url":
		return "must be a valid URL"
	case "oneof":
		return "value is not allowed"
	default:
		if tag == "" {
			return field + " is invalid"
		}
		return field + " is invalid (" + tag + ")"
	}
}

// toSnakeCase converts a Go-style field name into the snake_case JSON
// shape the rest of the API uses ("InputFileID" → "input_file_id",
// "Mode" → "mode"). Adjacent uppercase letters are treated as an acronym
// and collapse together — only a boundary between an upper and a lower
// letter (or vice-versa) emits an underscore.
func toSnakeCase(s string) string {
	if s == "" {
		return s
	}
	var b []byte
	r := []rune(s)
	for i, c := range r {
		isUpper := c >= 'A' && c <= 'Z'
		if isUpper && i > 0 {
			prevLower := r[i-1] >= 'a' && r[i-1] <= 'z'
			// "MyXMLValue" → boundary before "X" (lower→upper) AND
			// before "Value" (upper run followed by a lowercase).
			nextLower := i+1 < len(r) && r[i+1] >= 'a' && r[i+1] <= 'z'
			prevUpper := r[i-1] >= 'A' && r[i-1] <= 'Z'
			if prevLower || (prevUpper && nextLower) {
				b = append(b, '_')
			}
		}
		if isUpper {
			b = append(b, byte(c)+32)
		} else {
			b = append(b, byte(c))
		}
	}
	return string(b)
}
