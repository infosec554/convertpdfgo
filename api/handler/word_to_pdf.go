package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/queue"
	"convertpdfgo/pkg/timeouts"
)

// CreateWordToPDF godoc
// @Summary      Convert Word to PDF (async)
// @Tags         Word to PDF
// @Accept       json
// @Produce      json
// @Param        request body models.WordToPDFRequest true "Word to PDF"
// @Success      202 {object} map[string]string
// @Failure      400 {object} models.Response
// @Failure      500 {object} models.Response
// @Router       /api/pdf/word-to-pdf [post]
func (h *Handler) CreateWordToPDF(c *gin.Context) {
	var req models.WordToPDFRequest
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

	jobID, err := h.services.WordToPDF().Create(ctx, req, userID)
	if err != nil {
		handleResponse(c, h.log, "failed to create job", http.StatusInternalServerError, err.Error())
		return
	}

	task, opts, err := queue.NewSimpleTask(queue.TaskWordToPDF, jobID, queue.QueueDefault)
	if err != nil {
		handleResponse(c, h.log, "failed to build task", http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := h.queue.EnqueueContext(ctx, task, opts...); err != nil {
		handleResponse(c, h.log, "failed to enqueue task", http.StatusInternalServerError, err.Error())
		return
	}

	handleResponse(c, h.log, "word_to_pdf job queued", http.StatusAccepted, gin.H{"id": jobID, "status": "pending"})
}

// GetWordToPDFJob godoc
// @Summary      Get Word→PDF job status
// @Tags         Word to PDF
// @Produce      json
// @Param        id path string true "Job ID"
// @Success      200 {object} models.WordToPDFJob
// @Failure      404 {object} models.Response
// @Router       /api/pdf/word-to-pdf/{id} [get]
func (h *Handler) GetWordToPDFJob(c *gin.Context) {
	jobID := c.Param("id")

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	job, err := h.services.WordToPDF().GetByID(ctx, jobID)
	if err != nil {
		handleResponse(c, h.log, "job not found", http.StatusNotFound, err.Error())
		return
	}

	handleResponse(c, h.log, "job found", http.StatusOK, job)
}
