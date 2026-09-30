package handler

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"convertpdfgo/service/aicmd"
	"convertpdfgo/service/llm"
)

// AICommandRequest is the body for POST /api/ai/command.
//
// The catalogue comes from the caller rather than the server because the
// tool registry lives in the frontend package — one list, edited in one
// place, instead of a copy here that drifts the first time a tool is
// added. The model's answer is still checked against whatever was sent,
// so a caller cannot talk this endpoint into returning a slug it did
// not itself offer.
type AICommandRequest struct {
	Text      string      `json:"text"`
	FileNames []string    `json:"file_names"`
	Tools     []aicmd.Tool `json:"tools" binding:"required,min=1"`
}

// AICommand godoc
// @Router       /api/ai/command [POST]
// @Summary      Route a natural-language request to a tool
// @Description  Takes what the user typed plus the names of any attached files, and returns which tool to open.
// @Tags         ai
// @Accept       json
// @Produce      json
// @Param        request body AICommandRequest true "Command"
// @Success      200 {object} aicmd.Plan
// @Failure      400 {object} models.Response
// @Failure      503 {object} models.Response
func (h Handler) AICommand(c *gin.Context) {
	var req AICommandRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleResponse(c, h.log, "invalid request", http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(req.Text) == "" && len(req.FileNames) == 0 {
		handleResponse(c, h.log, "empty command", http.StatusBadRequest, "text or file_names required")
		return
	}

	provider, err := llm.Chatter()
	if err != nil {
		// No key configured is an operator problem, not a caller one —
		// 503 so the UI can say "not available" rather than "you did
		// something wrong".
		handleResponse(c, h.log, "ai unavailable", http.StatusServiceUnavailable, err.Error())
		return
	}

	// Routing is interactive; a caller waiting on this has a cursor
	// blinking at them.
	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()

	plan, err := aicmd.Route(ctx, provider, aicmd.Request{
		Text:      req.Text,
		FileNames: req.FileNames,
		Tools:     req.Tools,
	})
	if err != nil {
		if err == llm.ErrRateLimited {
			handleResponse(c, h.log, "rate limited", http.StatusTooManyRequests,
				"Too many requests just now — try again in a moment.")
			return
		}
		handleResponse(c, h.log, "routing failed", http.StatusServiceUnavailable, err.Error())
		return
	}

	handleResponse(c, h.log, "plan", http.StatusOK, plan)
}
