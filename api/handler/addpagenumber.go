package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/queue"
	"convertpdfgo/pkg/timeouts"
)

// CreateAddPageNumbersJob godoc
// @Router       /api/pdf/add-page-numbers [POST]
// @Summary      Add page numbers (async)
// @Tags         PDF
// @Accept       json
// @Produce      json
// @Param        data body models.AddPageNumbersRequest true "PDF and font settings"
// @Success      202 {object} map[string]string
func (h *Handler) CreateAddPageNumbersJob(c *gin.Context) {
	var req models.AddPageNumbersRequest
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

	jobID, err := h.services.AddPageNumber().Create(ctx, req, userID)
	if err != nil {
		handleResponse(c, h.log, "failed to create add-page-numbers job", http.StatusInternalServerError, err.Error())
		return
	}

	task, opts, err := queue.NewSimpleTask(queue.TaskAddPageNumber, jobID, queue.QueueDefault)
	if err != nil {
		handleResponse(c, h.log, "failed to build task", http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := h.queue.EnqueueContext(ctx, task, opts...); err != nil {
		handleResponse(c, h.log, "failed to enqueue task", http.StatusInternalServerError, err.Error())
		return
	}

	handleResponse(c, h.log, "add page numbers job queued", http.StatusAccepted, models.JobCreatedResponse{ID: jobID, Status: models.JobStatusPending})
}

// GetAddPageNumbersJob godoc
// @Router       /api/pdf/add-page-numbers/{id} [GET]
// @Summary      Get add page numbers job
// @Tags         PDF
// @Param        id path string true "Job ID"
// @Success      200 {object} models.AddPageNumberJob
func (h *Handler) GetAddPageNumbersJob(c *gin.Context) {
	id := c.Param("id")

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	job, err := h.services.AddPageNumber().GetByID(ctx, id)
	if err != nil {
		handleResponse(c, h.log, "job not found", http.StatusNotFound, err.Error())
		return
	}
	handleResponse(c, h.log, "job found", http.StatusOK, job)
}
