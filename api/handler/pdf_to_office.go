// HTTP handlers for /api/pdf/{pdf-to-html, pdf-to-ppt, pdf-to-excel}.
// All three follow the same shape as pdf_to_word.go (validate input,
// create the job, enqueue, return 202 + JobCreatedResponse). Kept in
// one file because the per-endpoint differences are mechanical and
// splitting them three ways added more navigation than it saved.
package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/queue"
	"convertpdfgo/pkg/timeouts"
)

// --- PDF → HTML -----------------------------------------------------------

// CreatePDFToHTMLJob godoc
// @Summary      PDF → HTML (async, LibreOffice)
// @Tags         PDF to HTML
// @Accept       json
// @Produce      json
// @Param        request body models.PDFToHTMLRequest true "PDF→HTML"
// @Success      202 {object} models.JobCreatedResponse
// @Router       /api/pdf/pdf-to-html [post]
func (h *Handler) CreatePDFToHTMLJob(c *gin.Context) {
	var req models.PDFToHTMLRequest
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

	jobID, err := h.services.PDFToHTML().Create(ctx, req, userID)
	if err != nil {
		handleResponse(c, h.log, "failed to create job", http.StatusInternalServerError, err.Error())
		return
	}
	task, opts, err := queue.NewSimpleTask(queue.TaskPDFToHTML, jobID, queue.QueueDefault)
	if err != nil {
		handleResponse(c, h.log, "failed to build task", http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := h.queue.EnqueueContext(ctx, task, opts...); err != nil {
		handleResponse(c, h.log, "failed to enqueue task", http.StatusInternalServerError, err.Error())
		return
	}
	handleResponse(c, h.log, "pdf_to_html job queued", http.StatusAccepted,
		models.JobCreatedResponse{ID: jobID, Status: models.JobStatusPending})
}

// GetPDFToHTMLJob godoc
// @Summary  PDF→HTML status
// @Tags     PDF to HTML
// @Param    id path string true "Job ID"
// @Success  200 {object} models.PDFToHTMLJob
// @Router   /api/pdf/pdf-to-html/{id} [get]
func (h *Handler) GetPDFToHTMLJob(c *gin.Context) {
	jobID := c.Param("id")
	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()
	job, err := h.services.PDFToHTML().GetByID(ctx, jobID)
	if err != nil {
		handleResponse(c, h.log, "job not found", http.StatusNotFound, err.Error())
		return
	}
	handleResponse(c, h.log, "job found", http.StatusOK, job)
}

// --- PDF → PPT ------------------------------------------------------------

// CreatePDFToPPTJob godoc
// @Summary      PDF → PowerPoint (async, LibreOffice impress)
// @Tags         PDF to PPT
// @Accept       json
// @Produce      json
// @Param        request body models.PDFToPPTRequest true "PDF→PPT"
// @Success      202 {object} models.JobCreatedResponse
// @Router       /api/pdf/pdf-to-ppt [post]
func (h *Handler) CreatePDFToPPTJob(c *gin.Context) {
	var req models.PDFToPPTRequest
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

	jobID, err := h.services.PDFToPPT().Create(ctx, req, userID)
	if err != nil {
		handleResponse(c, h.log, "failed to create job", http.StatusInternalServerError, err.Error())
		return
	}
	task, opts, err := queue.NewSimpleTask(queue.TaskPDFToPPT, jobID, queue.QueueDefault)
	if err != nil {
		handleResponse(c, h.log, "failed to build task", http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := h.queue.EnqueueContext(ctx, task, opts...); err != nil {
		handleResponse(c, h.log, "failed to enqueue task", http.StatusInternalServerError, err.Error())
		return
	}
	handleResponse(c, h.log, "pdf_to_ppt job queued", http.StatusAccepted,
		models.JobCreatedResponse{ID: jobID, Status: models.JobStatusPending})
}

// GetPDFToPPTJob godoc
// @Summary  PDF→PPT status
// @Tags     PDF to PPT
// @Param    id path string true "Job ID"
// @Success  200 {object} models.PDFToPPTJob
// @Router   /api/pdf/pdf-to-ppt/{id} [get]
func (h *Handler) GetPDFToPPTJob(c *gin.Context) {
	jobID := c.Param("id")
	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()
	job, err := h.services.PDFToPPT().GetByID(ctx, jobID)
	if err != nil {
		handleResponse(c, h.log, "job not found", http.StatusNotFound, err.Error())
		return
	}
	handleResponse(c, h.log, "job found", http.StatusOK, job)
}

// --- PDF → Excel ----------------------------------------------------------

// CreatePDFToExcelJob godoc
// @Summary      PDF → Excel (async; pdftotext -layout + excelize)
// @Tags         PDF to Excel
// @Accept       json
// @Produce      json
// @Param        request body models.PDFToExcelRequest true "PDF→Excel"
// @Success      202 {object} models.JobCreatedResponse
// @Router       /api/pdf/pdf-to-excel [post]
func (h *Handler) CreatePDFToExcelJob(c *gin.Context) {
	var req models.PDFToExcelRequest
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

	jobID, err := h.services.PDFToExcel().Create(ctx, req, userID)
	if err != nil {
		handleResponse(c, h.log, "failed to create job", http.StatusInternalServerError, err.Error())
		return
	}
	task, opts, err := queue.NewSimpleTask(queue.TaskPDFToExcel, jobID, queue.QueueDefault)
	if err != nil {
		handleResponse(c, h.log, "failed to build task", http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := h.queue.EnqueueContext(ctx, task, opts...); err != nil {
		handleResponse(c, h.log, "failed to enqueue task", http.StatusInternalServerError, err.Error())
		return
	}
	handleResponse(c, h.log, "pdf_to_excel job queued", http.StatusAccepted,
		models.JobCreatedResponse{ID: jobID, Status: models.JobStatusPending})
}

// GetPDFToExcelJob godoc
// @Summary  PDF→Excel status
// @Tags     PDF to Excel
// @Param    id path string true "Job ID"
// @Success  200 {object} models.PDFToExcelJob
// @Router   /api/pdf/pdf-to-excel/{id} [get]
func (h *Handler) GetPDFToExcelJob(c *gin.Context) {
	jobID := c.Param("id")
	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()
	job, err := h.services.PDFToExcel().GetByID(ctx, jobID)
	if err != nil {
		handleResponse(c, h.log, "job not found", http.StatusNotFound, err.Error())
		return
	}
	handleResponse(c, h.log, "job found", http.StatusOK, job)
}
