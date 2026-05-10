package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/queue"
	"convertpdfgo/pkg/timeouts"
)

// CreateJPGToPDF godoc
// @Router       /api/pdf/jpg-to-pdf [POST]
// @Summary      JPG → PDF (async)
// @Tags         jpg-to-pdf
// @Accept       json
// @Produce      json
// @Param        request body models.CreateJPGToPDFRequest true "JPG→PDF"
// @Success      202 {object} map[string]string
func (h Handler) CreateJPGToPDF(c *gin.Context) {
	var req models.CreateJPGToPDFRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleResponse(c, h.log, "invalid request", http.StatusBadRequest, err.Error())
		return
	}
	if len(req.InputFileIDs) == 0 {
		handleResponse(c, h.log, "no input files", http.StatusBadRequest, "input_file_ids required")
		return
	}

	var userID *string
	if uid := c.GetString("user_id"); uid != "" {
		userID = &uid
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	jobID, err := h.services.JPGToPDF().CreateJob(ctx, userID, req.InputFileIDs)
	if err != nil {
		handleResponse(c, h.log, "failed to create job", http.StatusInternalServerError, err.Error())
		return
	}

	task, opts, err := queue.NewSimpleTask(queue.TaskJPGToPDF, jobID, queue.QueueDefault)
	if err != nil {
		handleResponse(c, h.log, "failed to build task", http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := h.queue.EnqueueContext(ctx, task, opts...); err != nil {
		handleResponse(c, h.log, "failed to enqueue task", http.StatusInternalServerError, err.Error())
		return
	}

	handleResponse(c, h.log, "jpg_to_pdf job queued", http.StatusAccepted, gin.H{"id": jobID, "status": "pending"})
}

// GetJPGToPDFJob godoc
// @Router       /api/pdf/jpg-to-pdf/{id} [GET]
// @Summary      JPG→PDF status
// @Tags         jpg-to-pdf
// @Param        id path string true "Job ID"
// @Success      200 {object} models.JPGToPDFJob
func (h Handler) GetJPGToPDFJob(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		handleResponse(c, h.log, "missing job id", http.StatusBadRequest, "id is required")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	job, err := h.services.JPGToPDF().GetJobByID(ctx, id)
	if err != nil {
		handleResponse(c, h.log, "job not found", http.StatusNotFound, err.Error())
		return
	}
	handleResponse(c, h.log, "job fetched", http.StatusOK, job)
}
