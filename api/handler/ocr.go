// HTTP handlers for the OCR family of endpoints. All six follow the
// same shape — parse the request, optionally set user_id, create the
// job, enqueue, return 202 + JobCreatedResponse. Kept in one file so
// the mechanical differences (request struct, service method, task
// type) stay visible side by side.
package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/queue"
	"convertpdfgo/pkg/timeouts"
)

func optionalUserID(c *gin.Context) *string {
	if uid := c.GetString("user_id"); uid != "" {
		return &uid
	}
	return nil
}

// --- /api/pdf/ocr ----------------------------------------------------------

// CreateOCRJob godoc
// @Summary  Add a searchable text layer to a scanned PDF
// @Tags     OCR
// @Accept   json
// @Produce  json
// @Param    request body models.OCRRequest true "OCR job"
// @Success  202 {object} models.JobCreatedResponse
// @Router   /api/pdf/ocr [post]
func (h *Handler) CreateOCRJob(c *gin.Context) {
	var req models.OCRRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleResponse(c, h.log, "invalid request", http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()
	jobID, err := h.services.OCR().Create(ctx, req, optionalUserID(c))
	if err != nil {
		handleResponse(c, h.log, "failed to create job", http.StatusInternalServerError, err.Error())
		return
	}
	task, opts, err := queue.NewSimpleTask(queue.TaskOCR, jobID, queue.QueueDefault)
	if err != nil {
		handleResponse(c, h.log, "failed to build task", http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := h.queue.EnqueueContext(ctx, task, opts...); err != nil {
		handleResponse(c, h.log, "failed to enqueue task", http.StatusInternalServerError, err.Error())
		return
	}
	handleResponse(c, h.log, "ocr job queued", http.StatusAccepted,
		models.JobCreatedResponse{ID: jobID, Status: models.JobStatusPending})
}

// GetOCRJob godoc
// @Summary  OCR job status
// @Tags     OCR
// @Param    id path string true "Job ID"
// @Success  200 {object} models.OCRJob
// @Router   /api/pdf/ocr/{id} [get]
func (h *Handler) GetOCRJob(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()
	job, err := h.services.OCR().GetByID(ctx, c.Param("id"))
	if err != nil {
		handleResponse(c, h.log, "job not found", http.StatusNotFound, err.Error())
		return
	}
	handleResponse(c, h.log, "job found", http.StatusOK, job)
}

// --- /api/pdf/scan-to-pdf --------------------------------------------------

// CreateScanToPDFJob godoc
// @Summary  Build a searchable PDF from one or more photographed pages
// @Tags     OCR
// @Accept   json
// @Produce  json
// @Param    request body models.ScanToPDFRequest true "scan-to-pdf"
// @Success  202 {object} models.JobCreatedResponse
// @Router   /api/pdf/scan-to-pdf [post]
func (h *Handler) CreateScanToPDFJob(c *gin.Context) {
	var req models.ScanToPDFRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleResponse(c, h.log, "invalid request", http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()
	jobID, err := h.services.ScanToPDF().Create(ctx, req, optionalUserID(c))
	if err != nil {
		handleResponse(c, h.log, "failed to create job", http.StatusInternalServerError, err.Error())
		return
	}
	task, opts, err := queue.NewSimpleTask(queue.TaskScanToPDF, jobID, queue.QueueDefault)
	if err != nil {
		handleResponse(c, h.log, "failed to build task", http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := h.queue.EnqueueContext(ctx, task, opts...); err != nil {
		handleResponse(c, h.log, "failed to enqueue task", http.StatusInternalServerError, err.Error())
		return
	}
	handleResponse(c, h.log, "scan_to_pdf job queued", http.StatusAccepted,
		models.JobCreatedResponse{ID: jobID, Status: models.JobStatusPending})
}

// GetScanToPDFJob godoc
// @Summary  scan-to-pdf status
// @Tags     OCR
// @Param    id path string true "Job ID"
// @Success  200 {object} models.ScanToPDFJob
// @Router   /api/pdf/scan-to-pdf/{id} [get]
func (h *Handler) GetScanToPDFJob(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()
	job, err := h.services.ScanToPDF().GetByID(ctx, c.Param("id"))
	if err != nil {
		handleResponse(c, h.log, "job not found", http.StatusNotFound, err.Error())
		return
	}
	handleResponse(c, h.log, "job found", http.StatusOK, job)
}

// --- /api/pdf/ocr-extract-text --------------------------------------------

// CreateOCRExtractTextJob godoc
// @Summary  Extract plain text from a scanned PDF via OCR
// @Tags     OCR
// @Accept   json
// @Produce  json
// @Param    request body models.OCRExtractTextRequest true "ocr-extract-text"
// @Success  202 {object} models.JobCreatedResponse
// @Router   /api/pdf/ocr-extract-text [post]
func (h *Handler) CreateOCRExtractTextJob(c *gin.Context) {
	var req models.OCRExtractTextRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleResponse(c, h.log, "invalid request", http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()
	jobID, err := h.services.OCRExtractText().Create(ctx, req, optionalUserID(c))
	if err != nil {
		handleResponse(c, h.log, "failed to create job", http.StatusInternalServerError, err.Error())
		return
	}
	task, opts, err := queue.NewSimpleTask(queue.TaskOCRExtractText, jobID, queue.QueueDefault)
	if err != nil {
		handleResponse(c, h.log, "failed to build task", http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := h.queue.EnqueueContext(ctx, task, opts...); err != nil {
		handleResponse(c, h.log, "failed to enqueue task", http.StatusInternalServerError, err.Error())
		return
	}
	handleResponse(c, h.log, "ocr_extract_text job queued", http.StatusAccepted,
		models.JobCreatedResponse{ID: jobID, Status: models.JobStatusPending})
}

// GetOCRExtractTextJob godoc
// @Summary  ocr-extract-text status
// @Tags     OCR
// @Param    id path string true "Job ID"
// @Success  200 {object} models.OCRExtractTextJob
// @Router   /api/pdf/ocr-extract-text/{id} [get]
func (h *Handler) GetOCRExtractTextJob(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()
	job, err := h.services.OCRExtractText().GetByID(ctx, c.Param("id"))
	if err != nil {
		handleResponse(c, h.log, "job not found", http.StatusNotFound, err.Error())
		return
	}
	handleResponse(c, h.log, "job found", http.StatusOK, job)
}

// --- /api/pdf/image-to-text -----------------------------------------------

// CreateImageToTextJob godoc
// @Summary  Extract text from a single image (JPG/PNG)
// @Tags     OCR
// @Accept   json
// @Produce  json
// @Param    request body models.ImageToTextRequest true "image-to-text"
// @Success  202 {object} models.JobCreatedResponse
// @Router   /api/pdf/image-to-text [post]
func (h *Handler) CreateImageToTextJob(c *gin.Context) {
	var req models.ImageToTextRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleResponse(c, h.log, "invalid request", http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()
	jobID, err := h.services.ImageToText().Create(ctx, req, optionalUserID(c))
	if err != nil {
		handleResponse(c, h.log, "failed to create job", http.StatusInternalServerError, err.Error())
		return
	}
	task, opts, err := queue.NewSimpleTask(queue.TaskImageToText, jobID, queue.QueueDefault)
	if err != nil {
		handleResponse(c, h.log, "failed to build task", http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := h.queue.EnqueueContext(ctx, task, opts...); err != nil {
		handleResponse(c, h.log, "failed to enqueue task", http.StatusInternalServerError, err.Error())
		return
	}
	handleResponse(c, h.log, "image_to_text job queued", http.StatusAccepted,
		models.JobCreatedResponse{ID: jobID, Status: models.JobStatusPending})
}

// GetImageToTextJob godoc
// @Summary  image-to-text status
// @Tags     OCR
// @Param    id path string true "Job ID"
// @Success  200 {object} models.ImageToTextJob
// @Router   /api/pdf/image-to-text/{id} [get]
func (h *Handler) GetImageToTextJob(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()
	job, err := h.services.ImageToText().GetByID(ctx, c.Param("id"))
	if err != nil {
		handleResponse(c, h.log, "job not found", http.StatusNotFound, err.Error())
		return
	}
	handleResponse(c, h.log, "job found", http.StatusOK, job)
}

// --- /api/pdf/ocr-detect-lang ---------------------------------------------

// CreateOCRDetectLangJob godoc
// @Summary  Auto-detect the dominant script/language in a PDF or image
// @Tags     OCR
// @Accept   json
// @Produce  json
// @Param    request body models.OCRDetectLangRequest true "ocr-detect-lang"
// @Success  202 {object} models.JobCreatedResponse
// @Router   /api/pdf/ocr-detect-lang [post]
func (h *Handler) CreateOCRDetectLangJob(c *gin.Context) {
	var req models.OCRDetectLangRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleResponse(c, h.log, "invalid request", http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()
	jobID, err := h.services.OCRDetectLang().Create(ctx, req, optionalUserID(c))
	if err != nil {
		handleResponse(c, h.log, "failed to create job", http.StatusInternalServerError, err.Error())
		return
	}
	task, opts, err := queue.NewSimpleTask(queue.TaskOCRDetectLang, jobID, queue.QueueDefault)
	if err != nil {
		handleResponse(c, h.log, "failed to build task", http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := h.queue.EnqueueContext(ctx, task, opts...); err != nil {
		handleResponse(c, h.log, "failed to enqueue task", http.StatusInternalServerError, err.Error())
		return
	}
	handleResponse(c, h.log, "ocr_detect_lang job queued", http.StatusAccepted,
		models.JobCreatedResponse{ID: jobID, Status: models.JobStatusPending})
}

// GetOCRDetectLangJob godoc
// @Summary  ocr-detect-lang status (includes detected_lang + confidence)
// @Tags     OCR
// @Param    id path string true "Job ID"
// @Success  200 {object} models.OCRDetectLangJob
// @Router   /api/pdf/ocr-detect-lang/{id} [get]
func (h *Handler) GetOCRDetectLangJob(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()
	job, err := h.services.OCRDetectLang().GetByID(ctx, c.Param("id"))
	if err != nil {
		handleResponse(c, h.log, "job not found", http.StatusNotFound, err.Error())
		return
	}
	handleResponse(c, h.log, "job found", http.StatusOK, job)
}

// --- /api/pdf/ocr-structured ----------------------------------------------

// CreateOCRStructuredJob godoc
// @Summary  Run OCR and return per-word bounding boxes (hOCR wrapped in JSON)
// @Tags     OCR
// @Accept   json
// @Produce  json
// @Param    request body models.OCRStructuredRequest true "ocr-structured"
// @Success  202 {object} models.JobCreatedResponse
// @Router   /api/pdf/ocr-structured [post]
func (h *Handler) CreateOCRStructuredJob(c *gin.Context) {
	var req models.OCRStructuredRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleResponse(c, h.log, "invalid request", http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()
	jobID, err := h.services.OCRStructured().Create(ctx, req, optionalUserID(c))
	if err != nil {
		handleResponse(c, h.log, "failed to create job", http.StatusInternalServerError, err.Error())
		return
	}
	task, opts, err := queue.NewSimpleTask(queue.TaskOCRStructured, jobID, queue.QueueDefault)
	if err != nil {
		handleResponse(c, h.log, "failed to build task", http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := h.queue.EnqueueContext(ctx, task, opts...); err != nil {
		handleResponse(c, h.log, "failed to enqueue task", http.StatusInternalServerError, err.Error())
		return
	}
	handleResponse(c, h.log, "ocr_structured job queued", http.StatusAccepted,
		models.JobCreatedResponse{ID: jobID, Status: models.JobStatusPending})
}

// GetOCRStructuredJob godoc
// @Summary  ocr-structured status
// @Tags     OCR
// @Param    id path string true "Job ID"
// @Success  200 {object} models.OCRStructuredJob
// @Router   /api/pdf/ocr-structured/{id} [get]
func (h *Handler) GetOCRStructuredJob(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()
	job, err := h.services.OCRStructured().GetByID(ctx, c.Param("id"))
	if err != nil {
		handleResponse(c, h.log, "job not found", http.StatusNotFound, err.Error())
		return
	}
	handleResponse(c, h.log, "job found", http.StatusOK, job)
}
