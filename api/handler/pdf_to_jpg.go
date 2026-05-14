package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/queue"
	"convertpdfgo/pkg/timeouts"
)

// CreatePDFToJPG godoc
// @Summary      PDF → JPG (async)
// @Tags         pdf-to-jpg
// @Accept       json
// @Produce      json
// @Param        request body models.PDFToJPGRequest true "PDF file ID"
// @Success      202 {object} map[string]string
// @Router       /api/pdf/pdf-to-jpg [post]
func (h *Handler) CreatePDFToJPG(c *gin.Context) {
	var req models.PDFToJPGRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleResponse(c, h.log, "invalid request body", http.StatusBadRequest, err.Error())
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

	jobID, err := h.services.PDFToJPG().Create(ctx, req, userID)
	if err != nil {
		handleResponse(c, h.log, "failed to create job", http.StatusInternalServerError, err.Error())
		return
	}

	task, opts, err := queue.NewSimpleTask(queue.TaskPDFToJPG, jobID, c.GetString("user_id"), queue.QueueDefault)
	if err != nil {
		handleResponse(c, h.log, "failed to build task", http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := h.queue.EnqueueContext(ctx, task, opts...); err != nil {
		handleResponse(c, h.log, "failed to enqueue task", http.StatusInternalServerError, err.Error())
		return
	}

	handleResponse(c, h.log, "pdf_to_jpg job queued", http.StatusAccepted, models.JobCreatedResponse{ID: jobID, Status: models.JobStatusPending})
}

// GetPDFToJPG godoc
// @Summary      PDF→JPG status
// @Tags         pdf-to-jpg
// @Param        id path string true "Job ID"
// @Success      200 {object} models.PDFToJPGJob
// @Router       /api/pdf/pdf-to-jpg/{id} [get]
func (h *Handler) GetPDFToJPG(c *gin.Context) {
	jobID := c.Param("id")
	if jobID == "" {
		handleResponse(c, h.log, "job ID is required", http.StatusBadRequest, "id required")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	job, err := h.services.PDFToJPG().GetByID(ctx, jobID)
	if err != nil {
		handleResponse(c, h.log, "job not found", http.StatusNotFound, err.Error())
		return
	}
	handleResponse(c, h.log, "job fetched", http.StatusOK, job)
}
