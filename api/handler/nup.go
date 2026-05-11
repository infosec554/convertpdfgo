package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/queue"
	"convertpdfgo/pkg/timeouts"
)

// CreateNUpJob godoc
// @Router       /api/pdf/nup [POST]
// @Summary      N-up imposition (multi-page-per-sheet) — async
// @Tags         pdf-nup
// @Accept       json
// @Produce      json
// @Param        request body models.NUpRequest true "n-up job"
// @Success      202 {object} map[string]string
func (h *Handler) CreateNUpJob(c *gin.Context) {
	var req models.NUpRequest
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

	jobID, err := h.services.NUp().Create(ctx, req, userID)
	if err != nil {
		handleResponse(c, h.log, "failed to create nup job", http.StatusBadRequest, err.Error())
		return
	}
	task, opts, err := queue.NewSimpleTask(queue.TaskNUp, jobID, queue.QueueDefault)
	if err != nil {
		handleResponse(c, h.log, "failed to build task", http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := h.queue.EnqueueContext(ctx, task, opts...); err != nil {
		handleResponse(c, h.log, "failed to enqueue task", http.StatusInternalServerError, err.Error())
		return
	}
	handleResponse(c, h.log, "nup job queued", http.StatusAccepted, models.JobCreatedResponse{ID: jobID, Status: models.JobStatusPending})
}

// GetNUpJob godoc
// @Router       /api/pdf/nup/{id} [GET]
// @Summary      Get n-up job by ID
// @Tags         pdf-nup
// @Param        id path string true "n-up job ID"
// @Success      200 {object} models.NUpJob
func (h *Handler) GetNUpJob(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		handleResponse(c, h.log, "missing job id", http.StatusBadRequest, "id required")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()
	job, err := h.services.NUp().GetByID(ctx, id)
	if err != nil {
		handleResponse(c, h.log, "job not found", http.StatusNotFound, err.Error())
		return
	}
	handleResponse(c, h.log, "ok", http.StatusOK, job)
}
