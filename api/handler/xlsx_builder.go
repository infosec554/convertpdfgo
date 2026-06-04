package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/queue"
	"convertpdfgo/pkg/timeouts"
)

// CreateXLSXBuilderJob godoc
// @Router  /api/build/xlsx [POST]
// @Summary Generate an AI-built Excel workbook from a topic
// @Tags    ai-builder
func (h *Handler) CreateXLSXBuilderJob(c *gin.Context) {
	var req models.XLSXBuilderRequest
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

	jobID, err := h.services.XLSXBuilder().Create(ctx, req, userID)
	if err != nil {
		handleResponse(c, h.log, "failed to create xlsx builder job", http.StatusBadRequest, err.Error())
		return
	}
	task, opts, err := queue.NewSimpleTask(queue.TaskXLSXBuilder, jobID, c.GetString("user_id"), queue.QueueDefault)
	if err != nil {
		handleResponse(c, h.log, "failed to build task", http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := h.queue.EnqueueContext(ctx, task, opts...); err != nil {
		handleResponse(c, h.log, "failed to enqueue task", http.StatusInternalServerError, err.Error())
		return
	}
	handleResponse(c, h.log, "xlsx builder job queued", http.StatusAccepted,
		models.JobCreatedResponse{ID: jobID, Status: models.JobStatusPending})
}

// GetXLSXBuilderJob godoc
// @Router  /api/build/xlsx/{id} [GET]
// @Summary Fetch an xlsx builder job
func (h *Handler) GetXLSXBuilderJob(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		handleResponse(c, h.log, "missing job id", http.StatusBadRequest, "id required")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()
	job, err := h.services.XLSXBuilder().GetByID(ctx, id)
	if err != nil {
		handleResponse(c, h.log, "xlsx builder job not found", http.StatusNotFound, err.Error())
		return
	}
	handleResponse(c, h.log, "ok", http.StatusOK, job)
}
