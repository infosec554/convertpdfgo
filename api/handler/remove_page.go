package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/queue"
	"convertpdfgo/pkg/timeouts"
)

// CreateRemovePagesJob godoc
// @Router       /api/pdf/removepage [post]
// @Summary      Create remove pages job (async)
// @Tags         pdf-remove
// @Accept       json
// @Produce      json
// @Param        request body models.RemovePagesRequest true "remove pages job"
// @Success      202 {object} map[string]string
func (h Handler) CreateRemovePagesJob(c *gin.Context) {
	var req models.RemovePagesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleResponse(c, h.log, "invalid request body", http.StatusBadRequest, err.Error())
		return
	}

	var userID *string
	if uid := c.GetString("user_id"); uid != "" {
		userID = &uid
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	jobID, err := h.services.RemovePage().Create(ctx, req, userID)
	if err != nil {
		handleResponse(c, h.log, "failed to create remove pages job", http.StatusInternalServerError, err.Error())
		return
	}

	task, opts, err := queue.NewSimpleTask(queue.TaskRemovePage, jobID, queue.QueueDefault)
	if err != nil {
		handleResponse(c, h.log, "failed to build task", http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := h.queue.EnqueueContext(ctx, task, opts...); err != nil {
		handleResponse(c, h.log, "failed to enqueue task", http.StatusInternalServerError, err.Error())
		return
	}

	handleResponse(c, h.log, "remove pages job queued", http.StatusAccepted, gin.H{"id": jobID, "status": "pending"})
}

// GetRemovePagesJob godoc
// @Router       /api/pdf/removepage/{id} [GET]
// @Summary      Get remove pages job by ID
// @Tags         pdf-remove
// @Param        id path string true "remove job ID"
// @Success      200 {object} models.RemoveJob
func (h Handler) GetRemovePagesJob(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		handleResponse(c, h.log, "missing id", http.StatusBadRequest, "id required")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	job, err := h.services.RemovePage().GetByID(ctx, id)
	if err != nil {
		handleResponse(c, h.log, "failed to get remove job", http.StatusInternalServerError, err.Error())
		return
	}
	handleResponse(c, h.log, "remove job fetched", http.StatusOK, job)
}
