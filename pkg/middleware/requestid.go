package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const RequestIDHeader = "X-Request-ID"

// RequestID echoes an incoming X-Request-ID header or generates a new one,
// stuffs it into the request context, and reflects it on the response.
// Combined with structured logging this gives every request a single tag
// that follows it through API → worker → DB logs.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := c.GetHeader(RequestIDHeader)
		if rid == "" {
			rid = uuid.NewString()
		}
		c.Set("request_id", rid)
		c.Header(RequestIDHeader, rid)
		c.Next()
	}
}
