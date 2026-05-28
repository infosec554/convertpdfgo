package handler

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/queue"
	"convertpdfgo/pkg/timeouts"
	"convertpdfgo/service"
	"convertpdfgo/service/cv"
)

// CV handlers — every endpoint requires a signed-in user (AuthGate on
// the frontend + AuthorizerMiddleware on the route).

// CreateCVSession — POST /api/cv/sessions
func (h Handler) CreateCVSession(c *gin.Context) {
	var req models.CVCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleResponse(c, h.log, "invalid request", http.StatusBadRequest, err.Error())
		return
	}
	uid := c.GetString("user_id")
	if uid == "" {
		handleResponse(c, h.log, "sign-in required", http.StatusUnauthorized, "")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	sess, err := h.services.CV().CreateSession(ctx, uid, req)
	if err != nil {
		if errors.Is(err, cv.ErrCVDisabled) {
			handleResponse(c, h.log, "cv feature unavailable", http.StatusServiceUnavailable, err.Error())
			return
		}
		handleResponse(c, h.log, "create cv session failed", http.StatusInternalServerError, err.Error())
		return
	}

	// If this is an import session, enqueue the parser task right away
	// so the worker starts pulling text out of the uploaded file before
	// the user finishes navigating to the editor.
	if req.Mode == "import" {
		task, opts, terr := queue.NewSimpleTask(queue.TaskCVImport, sess.ID, uid, queue.QueueDefault)
		if terr != nil {
			handleResponse(c, h.log, "build task failed", http.StatusInternalServerError, terr.Error())
			return
		}
		enqCtx, enqCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer enqCancel()
		if _, terr := h.queue.EnqueueContext(enqCtx, task, opts...); terr != nil {
			handleResponse(c, h.log, "enqueue parser failed", http.StatusInternalServerError, terr.Error())
			return
		}
	}
	handleResponse(c, h.log, "cv session created", http.StatusCreated, sess)
}

// GetCVSession — GET /api/cv/sessions/:id
func (h Handler) GetCVSession(c *gin.Context) {
	id := c.Param("id")
	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()
	detail, err := h.services.CV().GetSession(ctx, id)
	if err != nil {
		if errors.Is(err, cv.ErrCVDisabled) {
			handleResponse(c, h.log, "cv feature unavailable", http.StatusServiceUnavailable, err.Error())
			return
		}
		handleResponse(c, h.log, "cv not found", http.StatusNotFound, err.Error())
		return
	}
	handleResponse(c, h.log, "cv fetched", http.StatusOK, detail)
}

// ListCVSessions — GET /api/cv/sessions
func (h Handler) ListCVSessions(c *gin.Context) {
	uid := c.GetString("user_id")
	if uid == "" {
		handleResponse(c, h.log, "sign-in required", http.StatusUnauthorized, "")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()
	sessions, err := h.services.CV().ListSessions(ctx, uid)
	if err != nil {
		handleResponse(c, h.log, "list cv failed", http.StatusInternalServerError, err.Error())
		return
	}
	handleResponse(c, h.log, "cv list", http.StatusOK, sessions)
}

// UpdateCVSession — PATCH /api/cv/sessions/:id
func (h Handler) UpdateCVSession(c *gin.Context) {
	id := c.Param("id")
	var patch models.CVUpdatePatch
	if err := c.ShouldBindJSON(&patch); err != nil {
		handleResponse(c, h.log, "invalid request", http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()
	if err := h.services.CV().UpdateSession(ctx, id, patch); err != nil {
		handleResponse(c, h.log, "update cv failed", http.StatusInternalServerError, err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

// DeleteCVSession — DELETE /api/cv/sessions/:id
func (h Handler) DeleteCVSession(c *gin.Context) {
	id := c.Param("id")
	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()
	if err := h.services.CV().DeleteSession(ctx, id); err != nil {
		handleResponse(c, h.log, "delete cv failed", http.StatusInternalServerError, err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

// ─── Sections ───────────────────────────────────────────────────────────

func (h Handler) AddCVSection(c *gin.Context) {
	cvID := c.Param("id")
	var req models.CVSectionUpsert
	if err := c.ShouldBindJSON(&req); err != nil {
		handleResponse(c, h.log, "invalid request", http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()
	sec, err := h.services.CV().AddSection(ctx, cvID, req)
	if err != nil {
		handleResponse(c, h.log, "add section failed", http.StatusInternalServerError, err.Error())
		return
	}
	handleResponse(c, h.log, "section added", http.StatusCreated, sec)
}

func (h Handler) UpdateCVSection(c *gin.Context) {
	sectionID := c.Param("sid")
	var body map[string]any
	if err := c.ShouldBindJSON(&body); err != nil {
		handleResponse(c, h.log, "invalid request", http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()
	if err := h.services.CV().UpdateSection(ctx, sectionID, body); err != nil {
		handleResponse(c, h.log, "update section failed", http.StatusInternalServerError, err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

func (h Handler) DeleteCVSection(c *gin.Context) {
	sectionID := c.Param("sid")
	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()
	if err := h.services.CV().DeleteSection(ctx, sectionID); err != nil {
		handleResponse(c, h.log, "delete section failed", http.StatusInternalServerError, err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

func (h Handler) ReorderCVSections(c *gin.Context) {
	cvID := c.Param("id")
	var req models.CVSectionReorder
	if err := c.ShouldBindJSON(&req); err != nil {
		handleResponse(c, h.log, "invalid request", http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()
	if err := h.services.CV().ReorderSections(ctx, cvID, req.Order); err != nil {
		handleResponse(c, h.log, "reorder failed", http.StatusInternalServerError, err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

// ─── AI ─────────────────────────────────────────────────────────────────

func (h Handler) ImproveCVField(c *gin.Context) {
	cvID := c.Param("id")
	uid := c.GetString("user_id")
	if uid == "" {
		handleResponse(c, h.log, "sign-in required", http.StatusUnauthorized, "")
		return
	}
	var req models.CVImproveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleResponse(c, h.log, "invalid request", http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	resp, err := h.services.CV().ImproveField(ctx, cvID, uid, req)
	if err != nil {
		if errors.Is(err, service.ErrChatDisabled) || errors.Is(err, cv.ErrCVDisabled) {
			handleResponse(c, h.log, "AI unavailable", http.StatusServiceUnavailable, err.Error())
			return
		}
		// Quota or rate errors come back with 429-ish wording — surface
		// them as 429 so the client can show a friendly retry-in message.
		handleResponse(c, h.log, "improve failed", http.StatusTooManyRequests, err.Error())
		return
	}
	handleResponse(c, h.log, "improve ok", http.StatusOK, resp)
}

func (h Handler) ParseCVVoice(c *gin.Context) {
	cvID := c.Param("id")
	var req models.CVVoiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleResponse(c, h.log, "invalid request", http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	resp, err := h.services.CV().ParseVoice(ctx, cvID, req)
	if err != nil {
		handleResponse(c, h.log, "voice parse failed", http.StatusInternalServerError, err.Error())
		return
	}
	handleResponse(c, h.log, "voice parsed", http.StatusOK, resp)
}

func (h Handler) TranslateCV(c *gin.Context) {
	cvID := c.Param("id")
	var req models.CVTranslateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleResponse(c, h.log, "invalid request", http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 90*time.Second)
	defer cancel()
	if err := h.services.CV().Translate(ctx, cvID, req.TargetLocale); err != nil {
		handleResponse(c, h.log, "translate failed", http.StatusInternalServerError, err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

func (h Handler) RunCVATSCheck(c *gin.Context) {
	cvID := c.Param("id")
	uid := c.GetString("user_id")
	if uid == "" {
		handleResponse(c, h.log, "sign-in required", http.StatusUnauthorized, "")
		return
	}
	var req models.CVATSRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleResponse(c, h.log, "invalid request", http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()
	run, err := h.services.CV().RunATSCheck(ctx, cvID, uid, req)
	if err != nil {
		handleResponse(c, h.log, "ats failed", http.StatusInternalServerError, err.Error())
		return
	}
	handleResponse(c, h.log, "ats ok", http.StatusOK, run)
}

func (h Handler) ListCVATSRuns(c *gin.Context) {
	cvID := c.Param("id")
	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()
	runs, err := h.services.CV().ListATSRuns(ctx, cvID)
	if err != nil {
		handleResponse(c, h.log, "list ats failed", http.StatusInternalServerError, err.Error())
		return
	}
	handleResponse(c, h.log, "ats history", http.StatusOK, runs)
}

// ─── Templates ──────────────────────────────────────────────────────────

func (h Handler) ListCVTemplates(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()
	tpls, err := h.services.CV().ListTemplates(ctx)
	if err != nil {
		handleResponse(c, h.log, "templates failed", http.StatusInternalServerError, err.Error())
		return
	}
	handleResponse(c, h.log, "templates", http.StatusOK, tpls)
}

// ─── Preview ────────────────────────────────────────────────────────────

func (h Handler) RenderCVPreview(c *gin.Context) {
	cvID := c.Param("id")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	html, err := h.services.CV().RenderPreviewHTML(ctx, cvID)
	if err != nil {
		handleResponse(c, h.log, "preview failed", http.StatusInternalServerError, err.Error())
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

// ─── Export ─────────────────────────────────────────────────────────────

func (h Handler) ExportCV(c *gin.Context) {
	cvID := c.Param("id")
	uid := c.GetString("user_id")
	if uid == "" {
		handleResponse(c, h.log, "sign-in required", http.StatusUnauthorized, "")
		return
	}
	var req models.CVExportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleResponse(c, h.log, "invalid request", http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()
	if _, err := h.services.CV().Export(ctx, cvID, req.Format, req.TemplateID); err != nil {
		handleResponse(c, h.log, "export failed", http.StatusInternalServerError, err.Error())
		return
	}
	// Enqueue the actual rendering as an asynq job — large CVs can take
	// a few seconds and we don't want the request to hold the gin worker.
	task, opts, terr := queue.NewSimpleTask(queue.TaskCVExport, cvID, uid, queue.QueueDefault)
	if terr != nil {
		handleResponse(c, h.log, "build task failed", http.StatusInternalServerError, terr.Error())
		return
	}
	enqCtx, enqCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer enqCancel()
	if _, terr := h.queue.EnqueueContext(enqCtx, task, opts...); terr != nil {
		handleResponse(c, h.log, "enqueue export failed", http.StatusInternalServerError, terr.Error())
		return
	}
	handleResponse(c, h.log, "export enqueued", http.StatusAccepted, gin.H{
		"cv_id": cvID,
		"format": req.Format,
		"hint": "poll GET /api/cv/sessions/" + cvID + " for last_pdf_id (PDF) or files list",
	})
}
