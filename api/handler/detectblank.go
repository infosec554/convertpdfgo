package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/queue"
	"convertpdfgo/pkg/timeouts"
)

// CreateDetectBlankPagesJob godoc
// @Router       /api/pdf/detect-blank [POST]
// @Security     ApiKeyAuth
// @Summary      Detect blank pages (async)
// @Tags         pdf-detect-blank
// @Accept       json
// @Produce      json
// @Param        inputFileID body models.DetectBlankPagesRequest true "Input PDF file ID"
// @Success      202 {object} map[string]string
func (h Handler) CreateDetectBlankPagesJob(c *gin.Context) {
	var req models.DetectBlankPagesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleResponse(c, h.log, "invalid request", http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	jobID, err := h.services.DetectBlank().Create(ctx, req, userPtr(c))
	if err != nil {
		handleResponse(c, h.log, "failed to create detect blank job", http.StatusInternalServerError, err.Error())
		return
	}

	task, opts, err := queue.NewSimpleTask(queue.TaskDetectBlank, jobID, queue.QueueLow)
	if err != nil {
		handleResponse(c, h.log, "failed to build task", http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := h.queue.EnqueueContext(ctx, task, opts...); err != nil {
		handleResponse(c, h.log, "failed to enqueue task", http.StatusInternalServerError, err.Error())
		return
	}

	handleResponse(c, h.log, "detect blank job queued", http.StatusAccepted, gin.H{"id": jobID, "status": "pending"})
}

// GetDetectBlankPagesJob godoc
// @Router       /api/pdf/detect-blank/{id} [GET]
// @Security     ApiKeyAuth
// @Summary      Get Detect Blank Pages Job
// @Tags         pdf-detect-blank
// @Param        id path string true "Job ID"
// @Success      200 {object} models.DetectBlankPagesJob
func (h Handler) GetDetectBlankPagesJob(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		handleResponse(c, h.log, "missing job id", http.StatusBadRequest, "id is required")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	job, err := h.services.DetectBlank().GetByID(ctx, id)
	if err != nil {
		handleResponse(c, h.log, "job not found", http.StatusNotFound, err.Error())
		return
	}
	handleResponse(c, h.log, "detect blank job fetched", http.StatusOK, job)
}
