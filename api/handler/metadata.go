package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/queue"
	"convertpdfgo/pkg/timeouts"
)

// CreateMetadataJob godoc
// @Router       /api/pdf/metadata [POST]
// @Summary      Edit PDF metadata (Title/Author/Subject/Keywords/Creator) — async
// @Tags         pdf-metadata
// @Accept       json
// @Produce      json
// @Param        request body models.MetadataRequest true "metadata job"
// @Success      202 {object} map[string]string
func (h *Handler) CreateMetadataJob(c *gin.Context) {
	var req models.MetadataRequest
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

	jobID, err := h.services.Metadata().Create(ctx, req, userID)
	if err != nil {
		handleResponse(c, h.log, "failed to create metadata job", http.StatusBadRequest, err.Error())
		return
	}
	task, opts, err := queue.NewSimpleTask(queue.TaskMetadata, jobID, queue.QueueLow)
	if err != nil {
		handleResponse(c, h.log, "failed to build task", http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := h.queue.EnqueueContext(ctx, task, opts...); err != nil {
		handleResponse(c, h.log, "failed to enqueue task", http.StatusInternalServerError, err.Error())
		return
	}
	handleResponse(c, h.log, "metadata job queued", http.StatusAccepted, gin.H{"id": jobID, "status": "pending"})
}

// GetMetadataJob godoc
// @Router       /api/pdf/metadata/{id} [GET]
// @Summary      Get metadata job by ID
// @Tags         pdf-metadata
// @Param        id path string true "metadata job ID"
// @Success      200 {object} models.MetadataJob
func (h *Handler) GetMetadataJob(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		handleResponse(c, h.log, "missing job id", http.StatusBadRequest, "id required")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()
	job, err := h.services.Metadata().GetByID(ctx, id)
	if err != nil {
		handleResponse(c, h.log, "job not found", http.StatusNotFound, err.Error())
		return
	}
	handleResponse(c, h.log, "ok", http.StatusOK, job)
}
