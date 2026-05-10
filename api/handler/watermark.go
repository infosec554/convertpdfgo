package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/queue"
	"convertpdfgo/pkg/timeouts"
)

// CreateWatermarkJob godoc
// @Router       /api/pdf/watermark [POST]
// @Summary      Add watermark (text or image) — async
// @Tags         pdf-watermark
// @Accept       json
// @Produce      json
// @Param        request body models.WatermarkRequest true "watermark job"
// @Success      202 {object} map[string]string
// @Failure      400 {object} models.Response
// @Failure      500 {object} models.Response
func (h *Handler) CreateWatermarkJob(c *gin.Context) {
	var req models.WatermarkRequest
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

	jobID, err := h.services.Watermark().Create(ctx, req, userID)
	if err != nil {
		handleResponse(c, h.log, "failed to create watermark job", http.StatusBadRequest, err.Error())
		return
	}
	task, opts, err := queue.NewSimpleTask(queue.TaskWatermark, jobID, queue.QueueDefault)
	if err != nil {
		handleResponse(c, h.log, "failed to build task", http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := h.queue.EnqueueContext(ctx, task, opts...); err != nil {
		handleResponse(c, h.log, "failed to enqueue task", http.StatusInternalServerError, err.Error())
		return
	}
	handleResponse(c, h.log, "watermark job queued", http.StatusAccepted, gin.H{"id": jobID, "status": "pending"})
}

// GetWatermarkJob godoc
// @Router       /api/pdf/watermark/{id} [GET]
// @Summary      Get watermark job by ID
// @Tags         pdf-watermark
// @Param        id path string true "watermark job ID"
// @Success      200 {object} models.WatermarkJob
// @Failure      404 {object} models.Response
func (h *Handler) GetWatermarkJob(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		handleResponse(c, h.log, "missing job id", http.StatusBadRequest, "id required")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()
	job, err := h.services.Watermark().GetByID(ctx, id)
	if err != nil {
		handleResponse(c, h.log, "watermark job not found", http.StatusNotFound, err.Error())
		return
	}
	handleResponse(c, h.log, "ok", http.StatusOK, job)
}
