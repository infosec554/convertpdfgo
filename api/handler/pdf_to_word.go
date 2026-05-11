package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/queue"
	"convertpdfgo/pkg/timeouts"
)

// CreatePDFToWordJob godoc
// @Summary      PDF → Word (async)
// @Tags         PDF to Word
// @Accept       json
// @Produce      json
// @Param        request body models.PDFToWordRequest true "PDF→Word"
// @Success      202 {object} map[string]string
// @Router       /api/pdf/pdf-to-word [post]
func (h *Handler) CreatePDFToWordJob(c *gin.Context) {
	var req models.PDFToWordRequest
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

	jobID, err := h.services.PDFToWord().Create(ctx, req, userID)
	if err != nil {
		handleResponse(c, h.log, "failed to create job", http.StatusInternalServerError, err.Error())
		return
	}

	task, opts, err := queue.NewSimpleTask(queue.TaskPDFToWord, jobID, queue.QueueDefault)
	if err != nil {
		handleResponse(c, h.log, "failed to build task", http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := h.queue.EnqueueContext(ctx, task, opts...); err != nil {
		handleResponse(c, h.log, "failed to enqueue task", http.StatusInternalServerError, err.Error())
		return
	}

	handleResponse(c, h.log, "pdf_to_word job queued", http.StatusAccepted, models.JobCreatedResponse{ID: jobID, Status: models.JobStatusPending})
}

// GetPDFToWordJob godoc
// @Summary      PDF→Word status
// @Tags         PDF to Word
// @Param        id path string true "Job ID"
// @Success      200 {object} models.PDFToWordJob
// @Router       /api/pdf/pdf-to-word/{id} [get]
func (h *Handler) GetPDFToWordJob(c *gin.Context) {
	jobID := c.Param("id")

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	job, err := h.services.PDFToWord().GetByID(ctx, jobID)
	if err != nil {
		handleResponse(c, h.log, "job not found", http.StatusNotFound, err.Error())
		return
	}
	handleResponse(c, h.log, "job found", http.StatusOK, job)
}
