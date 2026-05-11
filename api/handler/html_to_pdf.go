package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/queue"
	"convertpdfgo/pkg/timeouts"
)

// CreateHTMLToPDF godoc
// @Summary      HTML → PDF (async)
// @Tags         HTML to PDF
// @Accept       json
// @Produce      json
// @Param        data body models.CreateHTMLToPDFRequest true "HTML→PDF"
// @Success      202 {object} map[string]string
// @Router       /api/pdf/html-to-pdf [post]
func (h *Handler) CreateHTMLToPDF(c *gin.Context) {
	var req models.CreateHTMLToPDFRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleResponse(c, h.log, "invalid request body", http.StatusBadRequest, err.Error())
		return
	}
	if req.HTMLContent == "" {
		handleResponse(c, h.log, "html_content is required", http.StatusBadRequest, "empty html_content")
		return
	}

	var userID *string
	if uid := c.GetString("user_id"); uid != "" {
		userID = &uid
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	jobID, err := h.services.HTMLToPDF().Create(ctx, req, userID)
	if err != nil {
		handleResponse(c, h.log, "failed to create job", http.StatusInternalServerError, err.Error())
		return
	}

	task, opts, err := queue.NewSimpleTask(queue.TaskHTMLToPDF, jobID, queue.QueueDefault)
	if err != nil {
		handleResponse(c, h.log, "failed to build task", http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := h.queue.EnqueueContext(ctx, task, opts...); err != nil {
		handleResponse(c, h.log, "failed to enqueue task", http.StatusInternalServerError, err.Error())
		return
	}

	handleResponse(c, h.log, "html_to_pdf job queued", http.StatusAccepted, models.JobCreatedResponse{ID: jobID, Status: models.JobStatusPending})
}

// GetHTMLToPDFJob godoc
// @Summary      HTML→PDF status
// @Tags         HTML to PDF
// @Param        id path string true "Job ID"
// @Success      200 {object} models.HTMLToPDFJob
// @Router       /api/pdf/html-to-pdf/{id} [get]
func (h *Handler) GetHTMLToPDFJob(c *gin.Context) {
	jobID := c.Param("id")

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	job, err := h.services.HTMLToPDF().GetByID(ctx, jobID)
	if err != nil {
		handleResponse(c, h.log, "job not found", http.StatusNotFound, err.Error())
		return
	}
	handleResponse(c, h.log, "job found", http.StatusOK, job)
}
