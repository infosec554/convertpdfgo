package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/queue"
	"convertpdfgo/pkg/timeouts"
)

// CreateMergeJob godoc
// @Router /api/pdf/merge [post]
// @Summary      Create merge job (async)
// @Description  Saves the job and enqueues background processing. Poll GET /api/pdf/merge/{id} for status.
// @Tags         pdf-merge
// @Accept       json
// @Produce      json
// @Param        request body models.CreateMergeJobRequest true "merge job"
// @Success      202 {object} map[string]string
// @Failure      400 {object} models.Response
// @Failure      500 {object} models.Response
func (h Handler) CreateMergeJob(c *gin.Context) {
	var req models.CreateMergeJobRequest
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

	id, err := h.services.Merge().Create(ctx, userID, req.InputFileIDs)
	if err != nil {
		handleResponse(c, h.log, "failed to create merge job", http.StatusInternalServerError, err.Error())
		return
	}

	task, opts, err := queue.NewMergeTask(id, c.GetString("user_id"))
	if err != nil {
		handleResponse(c, h.log, "failed to build merge task", http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := h.queue.EnqueueContext(ctx, task, opts...); err != nil {
		handleResponse(c, h.log, "failed to enqueue merge task", http.StatusInternalServerError, err.Error())
		return
	}

	handleResponse(c, h.log, "merge job queued", http.StatusAccepted, models.JobCreatedResponse{ID: id, Status: models.JobStatusPending})
}

// GetMergeJob godoc
// @Router /api/pdf/merge/{id} [GET]
// @Summary      Get merge job status
// @Tags         pdf-merge
// @Accept       json
// @Produce      json
// @Param        id path string true "merge job ID"
// @Success      200 {object} models.MergeJob
// @Failure      404 {object} models.Response
// @Failure      500 {object} models.Response
func (h Handler) GetMergeJob(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		handleResponse(c, h.log, "missing job ID", http.StatusBadRequest, "id is required")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	job, err := h.services.Merge().GetByID(ctx, id)
	if err != nil {
		handleResponse(c, h.log, "merge job not found", http.StatusNotFound, err.Error())
		return
	}

	handleResponse(c, h.log, "merge job fetched", http.StatusOK, job)
}
