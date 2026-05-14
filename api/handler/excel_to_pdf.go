package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/queue"
	"convertpdfgo/pkg/timeouts"
)

// CreateExcelToPDF godoc
// @Summary      Excel → PDF (async)
// @Tags         Excel to PDF
// @Accept       json
// @Produce      json
// @Param        request body models.ExcelToPDFRequest true "Excel→PDF"
// @Success      202 {object} map[string]string
// @Router       /api/pdf/excel-to-pdf [post]
func (h *Handler) CreateExcelToPDF(c *gin.Context) {
	var req models.ExcelToPDFRequest
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

	jobID, err := h.services.ExcelToPDF().Create(ctx, req, userID)
	if err != nil {
		handleResponse(c, h.log, "failed to create job", http.StatusInternalServerError, err.Error())
		return
	}

	task, opts, err := queue.NewSimpleTask(queue.TaskExcelToPDF, jobID, c.GetString("user_id"), queue.QueueDefault)
	if err != nil {
		handleResponse(c, h.log, "failed to build task", http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := h.queue.EnqueueContext(ctx, task, opts...); err != nil {
		handleResponse(c, h.log, "failed to enqueue task", http.StatusInternalServerError, err.Error())
		return
	}

	handleResponse(c, h.log, "excel_to_pdf job queued", http.StatusAccepted, models.JobCreatedResponse{ID: jobID, Status: models.JobStatusPending})
}

// GetExcelToPDFJob godoc
// @Summary      Excel→PDF status
// @Tags         Excel to PDF
// @Param        id path string true "Job ID"
// @Success      200 {object} models.ExcelToPDFJob
// @Router       /api/pdf/excel-to-pdf/{id} [get]
func (h *Handler) GetExcelToPDFJob(c *gin.Context) {
	jobID := c.Param("id")

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	job, err := h.services.ExcelToPDF().GetByID(ctx, jobID)
	if err != nil {
		handleResponse(c, h.log, "job not found", http.StatusNotFound, err.Error())
		return
	}
	handleResponse(c, h.log, "job found", http.StatusOK, job)
}
