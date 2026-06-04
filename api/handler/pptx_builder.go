package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/queue"
	"convertpdfgo/pkg/timeouts"
)

// CreatePPTXBuilderJob godoc
// @Router  /api/build/pptx [POST]
// @Summary Generate an AI-built PowerPoint deck from a topic
// @Tags    ai-builder
// @Accept  json
// @Produce json
// @Param   request body models.PPTXBuilderRequest true "deck request"
// @Success 202 {object} models.JobCreatedResponse
func (h *Handler) CreatePPTXBuilderJob(c *gin.Context) {
	var req models.PPTXBuilderRequest
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

	jobID, err := h.services.PPTXBuilder().Create(ctx, req, userID)
	if err != nil {
		handleResponse(c, h.log, "failed to create pptx builder job", http.StatusBadRequest, err.Error())
		return
	}
	task, opts, err := queue.NewSimpleTask(queue.TaskPPTXBuilder, jobID, c.GetString("user_id"), queue.QueueDefault)
	if err != nil {
		handleResponse(c, h.log, "failed to build task", http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := h.queue.EnqueueContext(ctx, task, opts...); err != nil {
		handleResponse(c, h.log, "failed to enqueue task", http.StatusInternalServerError, err.Error())
		return
	}
	handleResponse(c, h.log, "pptx builder job queued", http.StatusAccepted,
		models.JobCreatedResponse{ID: jobID, Status: models.JobStatusPending})
}

// GetPPTXBuilderJob godoc
// @Router  /api/build/pptx/{id} [GET]
// @Summary Fetch a pptx builder job
// @Tags    ai-builder
// @Param   id path string true "job ID"
// @Success 200 {object} models.PPTXBuilderJob
func (h *Handler) GetPPTXBuilderJob(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		handleResponse(c, h.log, "missing job id", http.StatusBadRequest, "id required")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()
	job, err := h.services.PPTXBuilder().GetByID(ctx, id)
	if err != nil {
		handleResponse(c, h.log, "pptx builder job not found", http.StatusNotFound, err.Error())
		return
	}
	handleResponse(c, h.log, "ok", http.StatusOK, job)
}
