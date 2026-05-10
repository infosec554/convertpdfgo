package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/queue"
	"convertpdfgo/pkg/timeouts"
)

// CreateProtectJob godoc
// @Summary      Protect PDF with password (async)
// @Tags         PDF Security
// @Accept       json
// @Produce      json
// @Param        request body models.ProtectPDFRequest true "Protect"
// @Success      202 {object} map[string]string
// @Router       /api/pdf/protect [post]
func (h *Handler) CreateProtectJob(c *gin.Context) {
	var req models.ProtectPDFRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleResponse(c, h.log, "invalid request", http.StatusBadRequest, err.Error())
		return
	}
	if req.InputFileID == "" {
		handleResponse(c, h.log, "input_file_id required", http.StatusBadRequest, nil)
		return
	}
	if req.Password == "" {
		handleResponse(c, h.log, "password required", http.StatusBadRequest, nil)
		return
	}

	var userID *string
	if uid := c.GetString("user_id"); uid != "" {
		userID = &uid
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	jobID, err := h.services.Protect().Create(ctx, req, userID)
	if err != nil {
		handleResponse(c, h.log, "failed to create protect job", http.StatusInternalServerError, err.Error())
		return
	}

	task, opts, err := queue.NewSimpleTask(queue.TaskProtect, jobID, queue.QueueCritical)
	if err != nil {
		handleResponse(c, h.log, "failed to build task", http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := h.queue.EnqueueContext(ctx, task, opts...); err != nil {
		handleResponse(c, h.log, "failed to enqueue task", http.StatusInternalServerError, err.Error())
		return
	}

	handleResponse(c, h.log, "protect job queued", http.StatusAccepted, gin.H{"id": jobID, "status": "pending"})
}

// GetProtectJob godoc
// @Summary      Get Protect Job
// @Tags         PDF Security
// @Param        id path string true "Job ID"
// @Success      200 {object} models.ProtectPDFJob
// @Router       /api/pdf/protect/{id} [get]
func (h *Handler) GetProtectJob(c *gin.Context) {
	jobID := c.Param("id")

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	job, err := h.services.Protect().GetByID(ctx, jobID)
	if err != nil {
		handleResponse(c, h.log, "protect job not found", http.StatusNotFound, err.Error())
		return
	}
	handleResponse(c, h.log, "protect job found", http.StatusOK, job)
}
