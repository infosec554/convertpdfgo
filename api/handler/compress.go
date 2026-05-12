package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/queue"
	"convertpdfgo/pkg/timeouts"
)

// CreateCompressJob godoc
// @Router       /api/pdf/compress [POST]
// @Summary      Create compress job (async)
// @Tags         pdf-compress
// @Accept       json
// @Produce      json
// @Param        request body models.CompressRequest true "compress job"
// @Success      202 {object} map[string]string
// @Failure      400 {object} models.Response
// @Failure      500 {object} models.Response
func (h *Handler) CreateCompressJob(c *gin.Context) {
	var req models.CompressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleResponse(c, h.log, "invalid request", http.StatusBadRequest, err.Error())
		return
	}
	if req.Compression != models.Low && req.Compression != models.Medium && req.Compression != models.High {
		handleResponse(c, h.log, "invalid compression level", http.StatusBadRequest, "compression must be low, medium, or high")
		return
	}
	if req.InputFileID == "" {
		handleResponse(c, h.log, "no input file", http.StatusBadRequest, "input_file_id required")
		return
	}

	var userID *string
	if uid := c.GetString("user_id"); uid != "" {
		userID = &uid
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	jobID, err := h.services.Compress().Create(ctx, req, userID)
	if err != nil {
		handleResponse(c, h.log, "failed to create compress job", http.StatusInternalServerError, err.Error())
		return
	}

	task, opts, err := queue.NewSimpleTask(queue.TaskCompress, jobID, queue.QueueLow)
	if err != nil {
		handleResponse(c, h.log, "failed to build task", http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := h.queue.EnqueueContext(ctx, task, opts...); err != nil {
		handleResponse(c, h.log, "failed to enqueue task", http.StatusInternalServerError, err.Error())
		return
	}

	handleResponse(c, h.log, "compress job queued", http.StatusAccepted, models.JobCreatedResponse{ID: jobID, Status: models.JobStatusPending})
}

// GetCompressJob godoc
// @Router       /api/pdf/compress/{id} [GET]
// @Summary      Get compress job by ID
// @Tags         pdf-compress
// @Param        id path string true "compress job ID"
// @Success      200 {object} models.CompressJob
// @Failure      404 {object} models.Response
// @Failure      500 {object} models.Response
func (h *Handler) GetCompressJob(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		handleResponse(c, h.log, "missing job id", http.StatusBadRequest, "id required")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	job, err := h.services.Compress().GetByID(ctx, id)
	if err != nil {
		handleResponse(c, h.log, "failed to get compress job", http.StatusInternalServerError, err.Error())
		return
	}

	handleResponse(c, h.log, "compress job fetched", http.StatusOK, job)
}
