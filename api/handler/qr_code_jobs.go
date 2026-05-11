package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/queue"
	"convertpdfgo/pkg/timeouts"
)

// CreateQRCodeJob godoc
// @Router       /api/pdf/qr-code [POST]
// @Security     ApiKeyAuth
// @Summary      Add QR code to PDF (async)
// @Tags         pdf-qr-code
// @Accept       json
// @Produce      json
// @Param        request body models.CreateQRCodeRequest true "QR Code"
// @Success      202 {object} map[string]string
func (h Handler) CreateQRCodeJob(c *gin.Context) {
	var req models.CreateQRCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleResponse(c, h.log, "invalid request", http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	jobID, err := h.services.QRCode().Create(ctx, req, userPtr(c))
	if err != nil {
		handleResponse(c, h.log, "failed to create qr code job", http.StatusInternalServerError, err.Error())
		return
	}

	task, opts, err := queue.NewSimpleTask(queue.TaskQRCode, jobID, queue.QueueDefault)
	if err != nil {
		handleResponse(c, h.log, "failed to build task", http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := h.queue.EnqueueContext(ctx, task, opts...); err != nil {
		handleResponse(c, h.log, "failed to enqueue task", http.StatusInternalServerError, err.Error())
		return
	}

	handleResponse(c, h.log, "qr code job queued", http.StatusAccepted, models.JobCreatedResponse{ID: jobID, Status: models.JobStatusPending})
}

// GetQRCodeJob godoc
// @Router       /api/pdf/qr-code/{id} [GET]
// @Security     ApiKeyAuth
// @Summary      Get QR code job status
// @Tags         pdf-qr-code
// @Param        id path string true "Job ID"
// @Success      200 {object} models.QRCodeJob
func (h Handler) GetQRCodeJob(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		handleResponse(c, h.log, "missing job id", http.StatusBadRequest, "id is required")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	job, err := h.services.QRCode().GetByID(ctx, id)
	if err != nil {
		handleResponse(c, h.log, "job not found", http.StatusNotFound, err.Error())
		return
	}
	handleResponse(c, h.log, "qr code job fetched", http.StatusOK, job)
}
