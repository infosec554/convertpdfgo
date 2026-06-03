package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/queue"
	"convertpdfgo/pkg/timeouts"
)

// CreateSignJob godoc
// @Router       /api/pdf/sign [POST]
// @Summary      Place a signature image on a PDF page — async
// @Tags         pdf-sign
// @Accept       json
// @Produce      json
// @Param        request body models.SignRequest true "sign job"
// @Success      202 {object} map[string]string
// @Failure      400 {object} models.Response
// @Failure      500 {object} models.Response
func (h *Handler) CreateSignJob(c *gin.Context) {
	var req models.SignRequest
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

	jobID, err := h.services.Sign().Create(ctx, req, userID)
	if err != nil {
		handleResponse(c, h.log, "failed to create sign job", http.StatusBadRequest, err.Error())
		return
	}
	task, opts, err := queue.NewSimpleTask(queue.TaskSign, jobID, c.GetString("user_id"), queue.QueueDefault)
	if err != nil {
		handleResponse(c, h.log, "failed to build task", http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := h.queue.EnqueueContext(ctx, task, opts...); err != nil {
		handleResponse(c, h.log, "failed to enqueue task", http.StatusInternalServerError, err.Error())
		return
	}
	handleResponse(c, h.log, "sign job queued", http.StatusAccepted, models.JobCreatedResponse{ID: jobID, Status: models.JobStatusPending})
}

// GetSignJob godoc
// @Router       /api/pdf/sign/{id} [GET]
// @Summary      Get sign job by ID
// @Tags         pdf-sign
// @Param        id path string true "sign job ID"
// @Success      200 {object} models.SignJob
// @Failure      404 {object} models.Response
func (h *Handler) GetSignJob(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		handleResponse(c, h.log, "missing job id", http.StatusBadRequest, "id required")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()
	job, err := h.services.Sign().GetByID(ctx, id)
	if err != nil {
		handleResponse(c, h.log, "sign job not found", http.StatusNotFound, err.Error())
		return
	}
	handleResponse(c, h.log, "ok", http.StatusOK, job)
}
