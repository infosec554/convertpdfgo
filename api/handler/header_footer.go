package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/queue"
	"convertpdfgo/pkg/timeouts"
)

// AddHeaderFooter godoc
// @Router       /api/pdf/header-footer [POST]
// @Security     ApiKeyAuth
// @Summary      Add header/footer (async)
// @Tags         pdf-header-footer
// @Accept       json
// @Produce      json
// @Param        request body models.CreateAddHeaderFooterRequest true "Header/Footer params"
// @Success      202 {object} map[string]string
func (h Handler) AddHeaderFooter(c *gin.Context) {
	var req models.CreateAddHeaderFooterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleResponse(c, h.log, "invalid request", http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	jobID, err := h.services.AddHeaderFooter().Create(ctx, req, userPtr(c))
	if err != nil {
		handleResponse(c, h.log, "failed to create job", http.StatusInternalServerError, err.Error())
		return
	}

	task, opts, err := queue.NewSimpleTask(queue.TaskHeaderFooter, jobID, queue.QueueDefault)
	if err != nil {
		handleResponse(c, h.log, "failed to build task", http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := h.queue.EnqueueContext(ctx, task, opts...); err != nil {
		handleResponse(c, h.log, "failed to enqueue task", http.StatusInternalServerError, err.Error())
		return
	}

	handleResponse(c, h.log, "header/footer job queued", http.StatusAccepted, gin.H{"id": jobID, "status": "pending"})
}

// GetHeaderFooterJob godoc
// @Router       /api/pdf/header-footer/{id} [GET]
// @Security     ApiKeyAuth
// @Summary      Get header/footer job status
// @Tags         pdf-header-footer
// @Param        id path string true "Job ID"
// @Success      200 {object} models.AddHeaderFooterJob
func (h Handler) GetHeaderFooterJob(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		handleResponse(c, h.log, "missing job id", http.StatusBadRequest, "id is required")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	job, err := h.services.AddHeaderFooter().GetByID(ctx, id)
	if err != nil {
		handleResponse(c, h.log, "job not found", http.StatusNotFound, err.Error())
		return
	}
	handleResponse(c, h.log, "header/footer job fetched", http.StatusOK, job)
}
