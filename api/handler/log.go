package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// GetLogsByJobID godoc
// @Summary Get logs for a specific job
// @Description Get logs by job ID (for debugging or audit)
// @Tags logs
// @Accept json
// @Produce json
// @Param id path string true "Job ID"
// @Success 200 {array} models.Log
// @Failure 400 {object} models.Response
// @Failure 404 {object} models.Response
// @Failure 500 {object} models.Response
// @Router /api/logs/{id} [get]
// @Security ApiKeyAuth
func (h Handler) GetLogsByJobID(c *gin.Context) {
	jobID := c.Param("id")
	if jobID == "" {
		handleResponse(c, h.log, "missing job ID", http.StatusBadRequest, nil)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	logs, err := h.services.Log().GetLogsByJobID(ctx, jobID)
	if err != nil {
		handleResponse(c, h.log, "failed to fetch logs", http.StatusInternalServerError, err.Error())
		return
	}

	if len(logs) == 0 {
		handleResponse(c, h.log, "no logs found", http.StatusNotFound, nil)
		return
	}

	handleResponse(c, h.log, "logs retrieved successfully", http.StatusOK, logs)
}
