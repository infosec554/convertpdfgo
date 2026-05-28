package api

import (
	"github.com/gin-gonic/gin"

	"convertpdfgo/api/handler"
)

// registerCVRoutes mounts /api/cv/* — the AI Resume Builder feature.
//
// Every endpoint requires a signed-in user (AuthorizerMiddleware),
// matching the AuthGate on the frontend. The /api/cv/templates list is
// the only public read; everything else is per-user state.
func registerCVRoutes(r *gin.Engine, h handler.Handler) {
	// Templates list (public — used by both Hero "Templates" preview
	// and the editor modal, no auth required).
	r.GET("/api/cv/templates", h.ListCVTemplates)

	cv := r.Group("/api/cv", h.AuthorizerMiddleware)
	{
		// Sessions
		cv.POST("/sessions", h.CreateCVSession)
		cv.GET("/sessions", h.ListCVSessions)
		cv.GET("/sessions/:id", h.GetCVSession)
		cv.PATCH("/sessions/:id", h.UpdateCVSession)
		cv.DELETE("/sessions/:id", h.DeleteCVSession)

		// Sections
		cv.POST("/sessions/:id/sections", h.AddCVSection)
		cv.PATCH("/sessions/:id/sections/:sid", h.UpdateCVSection)
		cv.DELETE("/sessions/:id/sections/:sid", h.DeleteCVSection)
		cv.POST("/sessions/:id/sections/reorder", h.ReorderCVSections)

		// AI
		cv.POST("/sessions/:id/improve", h.ImproveCVField)
		cv.POST("/sessions/:id/voice", h.ParseCVVoice)
		cv.POST("/sessions/:id/translate", h.TranslateCV)

		// ATS
		cv.POST("/sessions/:id/ats-check", h.RunCVATSCheck)
		cv.POST("/sessions/:id/cover-letter", h.GenerateCVCoverLetter)
		cv.GET("/sessions/:id/ats-runs", h.ListCVATSRuns)

		// Preview + Export
		cv.GET("/sessions/:id/preview", h.RenderCVPreview)
		cv.POST("/sessions/:id/export", h.ExportCV)
	}
}
