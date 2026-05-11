package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/queue"
	"convertpdfgo/pkg/timeouts"
)

// CreateExtractJob godoc
// @Router       /api/pdf/extract [POST]
// @Summary      Create extract job (async)
// @Tags         pdf-extract
// @Accept       json
// @Produce      json
// @Param        request body models.ExtractPagesRequest true "extract request body"
// @Success      202 {object} map[string]string
func (h *Handler) CreateExtractJob(c *gin.Context) {
	var req models.ExtractPagesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleResponse(c, h.log, "invalid request body", http.StatusBadRequest, err.Error())
		return
	}
	if req.InputFileID == "" {
		handleResponse(c, h.log, "input_file_id required", http.StatusBadRequest, nil)
		return
	}

	var userID *string
	if uid := c.GetString("user_id"); uid != "" {
		userID = &uid
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	jobID, err := h.services.ExtractPage().Create(ctx, req, userID)
	if err != nil {
		handleResponse(c, h.log, "failed to create extract job", http.StatusInternalServerError, err.Error())
		return
	}

	task, opts, err := queue.NewSimpleTask(queue.TaskExtractPage, jobID, queue.QueueDefault)
	if err != nil {
		handleResponse(c, h.log, "failed to build task", http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := h.queue.EnqueueContext(ctx, task, opts...); err != nil {
		handleResponse(c, h.log, "failed to enqueue task", http.StatusInternalServerError, err.Error())
		return
	}

	handleResponse(c, h.log, "extract job queued", http.StatusAccepted, models.JobCreatedResponse{ID: jobID, Status: models.JobStatusPending})
}

// GetExtractJob godoc
// @Router       /api/pdf/extract/{id} [GET]
// @Summary      Get extract job by ID
// @Tags         pdf-extract
// @Param        id path string true "extract job ID"
// @Success      200 {object} models.ExtractJob
func (h *Handler) GetExtractJob(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		handleResponse(c, h.log, "missing id", http.StatusBadRequest, "id required")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	job, err := h.services.ExtractPage().GetByID(ctx, id)
	if err != nil {
		handleResponse(c, h.log, "failed to get extract job", http.StatusInternalServerError, err.Error())
		return
	}
	handleResponse(c, h.log, "extract job fetched", http.StatusOK, job)
}
