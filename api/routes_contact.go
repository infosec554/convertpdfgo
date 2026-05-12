package api

import (
	"github.com/gin-gonic/gin"

	"convertpdfgo/api/handler"
)

// registerContactRoutes mounts the user-facing contact submit. The
// admin half (list / get / reply / status / delete) lives in
// routes_admin.go under /admin/contacts/* so each audience stays in
// its own file.
//
// POST /api/contact is public — guests and registered users both
// post the same shape. userPtr inside the handler stamps user_id
// when a valid JWT is present so the admin inbox can correlate
// messages to a known account.
func registerContactRoutes(r *gin.Engine, h handler.Handler) {
	r.POST("/api/contact", h.OptionalAuthMiddleware, h.SubmitContact)
}
