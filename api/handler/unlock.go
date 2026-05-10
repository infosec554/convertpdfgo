package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/queue"
	"convertpdfgo/pkg/timeouts"
)

// CreateUnlockJob godoc
// @Summary      Unlock PDF (async)
// @Tags         PDF Unlock
// @Accept       json
// @Produce      json
// @Param        request body models.UnlockPDFRequest true "Unlock"
// @Success      202 {object} map[string]string
// @Router       /api/pdf/unlock [post]
func (h *Handler) CreateUnlockJob(c *gin.Context) {
	var req models.UnlockPDFRequest
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

	jobID, err := h.services.Unlock().Create(ctx, req, userID)
	if err != nil {
		handleResponse(c, h.log, "unlock job failed", http.StatusInternalServerError, err.Error())
		return
	}

	task, opts, err := queue.NewUnlockTask(jobID, req.Password)
	if err != nil {
		handleResponse(c, h.log, "failed to build task", http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := h.queue.EnqueueContext(ctx, task, opts...); err != nil {
		handleResponse(c, h.log, "failed to enqueue task", http.StatusInternalServerError, err.Error())
		return
	}

	handleResponse(c, h.log, "unlock job queued", http.StatusAccepted, gin.H{"id": jobID, "status": "pending"})
}

// GetUnlockJob godoc
// @Summary      Get Unlock Job status
// @Tags         PDF Unlock
// @Param        id path string true "Job ID"
// @Success      200 {object} models.UnlockPDFJob
// @Router       /api/pdf/unlock/{id} [get]
func (h *Handler) GetUnlockJob(c *gin.Context) {
	jobID := c.Param("id")

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	job, err := h.services.Unlock().GetByID(ctx, jobID)
	if err != nil {
		handleResponse(c, h.log, "unlock job not found", http.StatusNotFound, err.Error())
		return
	}
	handleResponse(c, h.log, "unlock job found", http.StatusOK, job)
}
