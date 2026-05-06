package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// BodyLimit caps the maximum bytes any handler may read from the request
// body. Wraps Request.Body in http.MaxBytesReader so attempts to read more
// fail fast — protects against memory exhaustion via crafted payloads.
//
// The handler-level upload size check (guest 20MB / registered 30MB) still
// runs separately; this is the global hard ceiling.
func BodyLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
	}
}
