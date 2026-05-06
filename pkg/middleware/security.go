package middleware

import "github.com/gin-gonic/gin"

// SecurityHeaders sets a baseline set of defensive HTTP response headers.
// They cost nothing and prevent common low-effort attacks (clickjacking,
// MIME sniffing, leaking referrers, downgrades to HTTP).
//
// HSTS is only sent when the request actually arrived over TLS, otherwise
// it would lock dev tooling out.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("X-XSS-Protection", "0") // modern guidance: rely on CSP, disable legacy filter
		c.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

		if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		c.Next()
	}
}
