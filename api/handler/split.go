package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/queue"
	"convertpdfgo/pkg/timeouts"
)

// CreateSplitJob godoc
// @Router       /api/pdf/split [post]
// @Summary      Create split job (async)
// @Tags         pdf-split
// @Accept       json
// @Produce      json
// @Param        request body models.CreateSplitJobRequest true "split job"
// @Success      202 {object} map[string]string
func (h Handler) CreateSplitJob(c *gin.Context) {
	var req models.CreateSplitJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleResponse(c, h.log, "invalid request body", http.StatusBadRequest, err.Error())
		return
	}

	var userID *string
	if val, ok := c.Get("user_id"); ok {
		if strID, ok := val.(string); ok && strID != "" {
			userID = &strID
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	jobID, err := h.services.Split().Create(ctx, req, userID)
	if err != nil {
		handleResponse(c, h.log, "failed to create split job", http.StatusInternalServerError, err.Error())
		return
	}

	task, opts, err := queue.NewSimpleTask(queue.TaskSplit, jobID, queue.QueueDefault)
	if err != nil {
		handleResponse(c, h.log, "failed to build task", http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := h.queue.EnqueueContext(ctx, task, opts...); err != nil {
		handleResponse(c, h.log, "failed to enqueue task", http.StatusInternalServerError, err.Error())
		return
	}

	handleResponse(c, h.log, "split job queued", http.StatusAccepted, models.JobCreatedResponse{ID: jobID, Status: models.JobStatusPending})
}

// GetSplitJob godoc
// @Router /api/pdf/split/{id} [get]
// @Summary      Get split job by ID
// @Tags         pdf-split
// @Param        id path string true "Job ID"
// @Success      200 {object} models.SplitJob
func (h Handler) GetSplitJob(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		handleResponse(c, h.log, "missing id", http.StatusBadRequest, "id is required")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	job, err := h.services.Split().GetByID(ctx, id)
	if err != nil {
		handleResponse(c, h.log, "failed to get split job", http.StatusInternalServerError, err.Error())
		return
	}
	handleResponse(c, h.log, "split job fetched", http.StatusOK, job)
}
