package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/queue"
	"convertpdfgo/pkg/timeouts"
)

// CreateExtractTextJob godoc
// @Router       /api/pdf/extract-text [POST]
// @Summary      Extract text (PDF → TXT/JSON) — async
// @Tags         pdf-extract-text
// @Accept       json
// @Produce      json
// @Param        request body models.ExtractTextRequest true "extract-text job"
// @Success      202 {object} map[string]string
func (h *Handler) CreateExtractTextJob(c *gin.Context) {
	var req models.ExtractTextRequest
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

	jobID, err := h.services.ExtractText().Create(ctx, req, userID)
	if err != nil {
		handleResponse(c, h.log, "failed to create extract_text job", http.StatusBadRequest, err.Error())
		return
	}
	task, opts, err := queue.NewSimpleTask(queue.TaskExtractText, jobID, queue.QueueLow)
	if err != nil {
		handleResponse(c, h.log, "failed to build task", http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := h.queue.EnqueueContext(ctx, task, opts...); err != nil {
		handleResponse(c, h.log, "failed to enqueue task", http.StatusInternalServerError, err.Error())
		return
	}
	handleResponse(c, h.log, "extract_text job queued", http.StatusAccepted, gin.H{"id": jobID, "status": "pending"})
}

// GetExtractTextJob godoc
// @Router       /api/pdf/extract-text/{id} [GET]
// @Summary      Get extract-text job by ID
// @Tags         pdf-extract-text
// @Param        id path string true "extract-text job ID"
// @Success      200 {object} models.ExtractTextJob
func (h *Handler) GetExtractTextJob(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		handleResponse(c, h.log, "missing job id", http.StatusBadRequest, "id required")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()
	job, err := h.services.ExtractText().GetByID(ctx, id)
	if err != nil {
		handleResponse(c, h.log, "job not found", http.StatusNotFound, err.Error())
		return
	}
	handleResponse(c, h.log, "ok", http.StatusOK, job)
}
