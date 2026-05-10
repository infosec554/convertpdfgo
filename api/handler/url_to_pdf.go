package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/queue"
	"convertpdfgo/pkg/timeouts"
)

// CreateURLToPDFJob godoc
// @Router       /api/pdf/url-to-pdf [POST]
// @Summary      Render URL → PDF (Gotenberg chromium) — async
// @Tags         pdf-url-to-pdf
// @Accept       json
// @Produce      json
// @Param        request body models.URLToPDFRequest true "url-to-pdf job"
// @Success      202 {object} map[string]string
func (h *Handler) CreateURLToPDFJob(c *gin.Context) {
	var req models.URLToPDFRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleResponse(c, h.log, "invalid request", http.StatusBadRequest, err.Error())
		return
	}
	var userID *string
	if uid := c.GetString("user_id"); uid != "" {
		userID = &uid
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	jobID, err := h.services.URLToPDF().Create(ctx, req, userID)
	if err != nil {
		handleResponse(c, h.log, "failed to create url_to_pdf job", http.StatusBadRequest, err.Error())
		return
	}
	task, opts, err := queue.NewSimpleTask(queue.TaskURLToPDF, jobID, queue.QueueDefault)
	if err != nil {
		handleResponse(c, h.log, "failed to build task", http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := h.queue.EnqueueContext(ctx, task, opts...); err != nil {
		handleResponse(c, h.log, "failed to enqueue task", http.StatusInternalServerError, err.Error())
		return
	}
	handleResponse(c, h.log, "url_to_pdf job queued", http.StatusAccepted, gin.H{"id": jobID, "status": "pending"})
}

// GetURLToPDFJob godoc
// @Router       /api/pdf/url-to-pdf/{id} [GET]
// @Summary      Get url-to-pdf job by ID
// @Tags         pdf-url-to-pdf
// @Param        id path string true "url-to-pdf job ID"
// @Success      200 {object} models.URLToPDFJob
func (h *Handler) GetURLToPDFJob(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		handleResponse(c, h.log, "missing job id", http.StatusBadRequest, "id required")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()
	job, err := h.services.URLToPDF().GetByID(ctx, id)
	if err != nil {
		handleResponse(c, h.log, "job not found", http.StatusNotFound, err.Error())
		return
	}
	handleResponse(c, h.log, "ok", http.StatusOK, job)
}
