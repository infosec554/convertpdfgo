package api

import (
	"github.com/gin-gonic/gin"

	"convertpdfgo/api/handler"
)

// registerChatRoutes mounts /api/chat/* — the PDF Chat AI feature.
//
// Sessions and messages are accessible to both guests (created with the
// uploader's IP/session) and signed-in users (linked to their account).
// We use OptionalAuthMiddleware so the handler sees user_id when a token
// is present and leaves it empty otherwise.
//
// D6/D7 will add POST /api/chat/sessions/<id>/messages (SSE streaming)
// and GET /messages (history). For now we expose just the session CRUD.
func registerChatRoutes(r *gin.Engine, h handler.Handler) {
	chat := r.Group("/api/chat", h.OptionalAuthMiddleware)
	{
		chat.POST("/sessions", h.CreateChatSession)
		chat.GET("/sessions", h.ListChatSessions)
		chat.GET("/sessions/:id", h.GetChatSession)
		chat.DELETE("/sessions/:id", h.DeleteChatSession)

		// Chat messages — SSE streaming POST + history GET.
		chat.POST("/sessions/:id/messages", h.SendChatMessage)
		chat.GET("/sessions/:id/messages", h.ListChatMessages)
	}
}
