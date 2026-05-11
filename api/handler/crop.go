package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/queue"
	"convertpdfgo/pkg/timeouts"
)

// CreateCropJob godoc
// @Summary      Crop PDF (async)
// @Tags         PDF Edit
// @Accept       json
// @Produce      json
// @Param        request body models.CropPDFRequest true "Crop"
// @Success      202 {object} map[string]string
// @Router       /api/pdf/crop [post]
func (h *Handler) CreateCropJob(c *gin.Context) {
	var req models.CropPDFRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleResponse(c, h.log, "invalid request", http.StatusBadRequest, err.Error())
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

	jobID, err := h.services.Crop().Create(ctx, req, userID)
	if err != nil {
		handleResponse(c, h.log, "crop job failed", http.StatusInternalServerError, err.Error())
		return
	}

	task, opts, err := queue.NewSimpleTask(queue.TaskCrop, jobID, queue.QueueDefault)
	if err != nil {
		handleResponse(c, h.log, "failed to build task", http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := h.queue.EnqueueContext(ctx, task, opts...); err != nil {
		handleResponse(c, h.log, "failed to enqueue task", http.StatusInternalServerError, err.Error())
		return
	}

	handleResponse(c, h.log, "crop job queued", http.StatusAccepted, models.JobCreatedResponse{ID: jobID, Status: models.JobStatusPending})
}

// GetCropJob godoc
// @Summary      Get Crop Job
// @Tags         PDF Edit
// @Param        id path string true "Job ID"
// @Success      200 {object} models.CropPDFJob
// @Router       /api/pdf/crop/{id} [get]
func (h *Handler) GetCropJob(c *gin.Context) {
	jobID := c.Param("id")

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	job, err := h.services.Crop().GetByID(ctx, jobID)
	if err != nil {
		handleResponse(c, h.log, "crop job not found", http.StatusNotFound, err.Error())
		return
	}
	handleResponse(c, h.log, "crop job found", http.StatusOK, job)
}
