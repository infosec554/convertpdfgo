package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// CORSConfig controls allowed origins. Empty AllowOrigins ⇒ "*", which is
// fine for dev / public APIs but should be tightened in production.
type CORSConfig struct {
	AllowOrigins     []string
	AllowMethods     []string
	AllowHeaders     []string
	ExposeHeaders    []string
	AllowCredentials bool
	MaxAgeSeconds    int
}

// DefaultCORS returns a permissive config suitable for development. Override
// AllowOrigins for production.
func DefaultCORS() CORSConfig {
	return CORSConfig{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowHeaders:     []string{"Authorization", "Content-Type", "X-Requested-With", "X-Request-ID"},
		ExposeHeaders:    []string{"X-Request-ID", "Content-Length"},
		AllowCredentials: false, // can't combine with "*" origin per spec
		MaxAgeSeconds:    600,
	}
}

// CORS returns a Gin middleware that emits the right Access-Control-* headers
// and short-circuits OPTIONS preflight requests.
func CORS(cfg CORSConfig) gin.HandlerFunc {
	allowAll := len(cfg.AllowOrigins) == 0 || (len(cfg.AllowOrigins) == 1 && cfg.AllowOrigins[0] == "*")
	allowed := make(map[string]struct{}, len(cfg.AllowOrigins))
	for _, o := range cfg.AllowOrigins {
		allowed[o] = struct{}{}
	}

	methods := strings.Join(cfg.AllowMethods, ", ")
	headers := strings.Join(cfg.AllowHeaders, ", ")
	expose := strings.Join(cfg.ExposeHeaders, ", ")
	maxAge := "600"
	if cfg.MaxAgeSeconds > 0 {
		maxAge = itoa(cfg.MaxAgeSeconds)
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" {
			c.Next()
			return
		}

		switch {
		case allowAll:
			c.Header("Access-Control-Allow-Origin", "*")
		default:
			if _, ok := allowed[origin]; ok {
				c.Header("Access-Control-Allow-Origin", origin)
				c.Header("Vary", "Origin")
			}
		}

		if cfg.AllowCredentials && !allowAll {
			c.Header("Access-Control-Allow-Credentials", "true")
		}
		if methods != "" {
			c.Header("Access-Control-Allow-Methods", methods)
		}
		if headers != "" {
			c.Header("Access-Control-Allow-Headers", headers)
		}
		if expose != "" {
			c.Header("Access-Control-Expose-Headers", expose)
		}
		c.Header("Access-Control-Max-Age", maxAge)

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
