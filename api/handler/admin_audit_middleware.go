package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"convertpdfgo/api/models"
)

// AdminAuditMiddleware writes one auth_audit row per /admin/* request that
// mutates state (POST/PUT/PATCH/DELETE). Read-only GETs are skipped to
// keep the table small — explicit per-handler audit calls (e.g. role
// change) still fire for those.
//
// We record only metadata (method, path, status, IP, UA, request_id) —
// never the request body, since admin payloads can carry passwords or
// other secrets.
func (h Handler) AdminAuditMiddleware(c *gin.Context) {
	// Run handler first so we can capture the response status.
	c.Next()

	switch c.Request.Method {
	case "POST", "PUT", "PATCH", "DELETE":
		// fall through and audit
	default:
		return
	}

	actor, _ := c.Get("user_id")
	actorStr, _ := actor.(string)

	// Build a compact event name "admin_<METHOD>" so admins filtering by
	// event in the audit table get a useful bucket without a dozen
	// hardcoded constants.
	evt := "admin_" + c.Request.Method

	rid, _ := c.Get("request_id")
	ridStr, _ := rid.(string)

	// Use the audit service so the write is async + retried in goroutine.
	var actorPtr *string
	if actorStr != "" {
		actorPtr = &actorStr
	}
	h.services.Audit().Record(c.Request.Context(), models.AuthAudit{
		UserID:    actorPtr,
		Email:     "",
		Event:     evt,
		Provider:  "",
		IP:        c.ClientIP(),
		UserAgent: c.GetHeader("User-Agent") + " | path=" + c.Request.URL.Path + " | status=" + strconv.Itoa(c.Writer.Status()) + " | rid=" + ridStr,
	})
}
