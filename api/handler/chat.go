package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/queue"
	"convertpdfgo/pkg/timeouts"
	"convertpdfgo/service"
	"convertpdfgo/service/llm"
)

// CreateChatSession godoc
// @Router       /api/chat/sessions [POST]
// @Summary      Start a PDF chat session
// @Description  Creates a new chat session for an already-uploaded PDF and
// @Description  kicks off async indexing (extract → chunk → embed → save).
// @Description  Returns immediately with status="indexing"; the client should
// @Description  poll GET /api/chat/sessions/<id> until status="ready".
// @Tags         chat
// @Accept       json
// @Produce      json
// @Param        request body models.ChatCreateSessionRequest true "Session create"
// @Success      201 {object} models.ChatSession
// @Failure      400 {object} models.Response
// @Failure      503 {object} models.Response  "Chat feature disabled (no LLM key)"
func (h Handler) CreateChatSession(c *gin.Context) {
	var req models.ChatCreateSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleResponse(c, h.log, "invalid request", http.StatusBadRequest, err.Error())
		return
	}
	if req.FileID == "" {
		handleResponse(c, h.log, "file_id required", http.StatusBadRequest, "file_id is required")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	sess, err := h.services.Chat().CreateSession(ctx, req.FileID, userPtr(c))
	if err != nil {
		if errors.Is(err, service.ErrChatDisabled) {
			handleResponse(c, h.log, "chat feature unavailable", http.StatusServiceUnavailable, err.Error())
			return
		}
		handleResponse(c, h.log, "failed to create chat session", http.StatusInternalServerError, err.Error())
		return
	}

	// Enqueue the indexing task. If the enqueue fails we still keep the
	// session row (status="indexing") so the user can retry — better
	// than orphaning the row with an inconsistent state.
	enqCtx, enqCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer enqCancel()
	task, opts, err := queue.NewSimpleTask(queue.TaskChatIndex, sess.ID, c.GetString("user_id"), queue.QueueDefault)
	if err != nil {
		handleResponse(c, h.log, "failed to build task", http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := h.queue.EnqueueContext(enqCtx, task, opts...); err != nil {
		handleResponse(c, h.log, "failed to enqueue indexing", http.StatusInternalServerError, err.Error())
		return
	}

	handleResponse(c, h.log, "chat session created", http.StatusCreated, sess)
}

// GetChatSession godoc
// @Router       /api/chat/sessions/{id} [GET]
// @Summary      Get chat session detail + suggested questions
// @Tags         chat
// @Param        id path string true "Session ID"
// @Success      200 {object} models.ChatSessionDetail
// @Failure      404 {object} models.Response
func (h Handler) GetChatSession(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		handleResponse(c, h.log, "missing session id", http.StatusBadRequest, "id is required")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	detail, err := h.services.Chat().GetSession(ctx, id)
	if err != nil {
		if errors.Is(err, service.ErrChatDisabled) {
			handleResponse(c, h.log, "chat feature unavailable", http.StatusServiceUnavailable, err.Error())
			return
		}
		handleResponse(c, h.log, "session not found", http.StatusNotFound, err.Error())
		return
	}
	handleResponse(c, h.log, "session fetched", http.StatusOK, detail)
}

// ListChatSessions godoc
// @Router       /api/chat/sessions [GET]
// @Summary      List the signed-in user's chat sessions
// @Description  Returns up to 50 most-recently-updated sessions. Guest
// @Description  callers get an empty list — we don't expose other guests'
// @Description  sessions even if they share an IP.
// @Tags         chat
// @Success      200 {array} models.ChatSession
func (h Handler) ListChatSessions(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	sessions, err := h.services.Chat().ListSessions(ctx, userPtr(c))
	if err != nil {
		if errors.Is(err, service.ErrChatDisabled) {
			handleResponse(c, h.log, "chat feature unavailable", http.StatusServiceUnavailable, err.Error())
			return
		}
		handleResponse(c, h.log, "failed to list sessions", http.StatusInternalServerError, err.Error())
		return
	}
	handleResponse(c, h.log, "sessions fetched", http.StatusOK, sessions)
}

// DeleteChatSession godoc
// @Router       /api/chat/sessions/{id} [DELETE]
// @Summary      Delete a chat session and its messages/chunks
// @Tags         chat
// @Param        id path string true "Session ID"
// @Success      204
func (h Handler) DeleteChatSession(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		handleResponse(c, h.log, "missing session id", http.StatusBadRequest, "id is required")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	if err := h.services.Chat().DeleteSession(ctx, id); err != nil {
		if errors.Is(err, service.ErrChatDisabled) {
			handleResponse(c, h.log, "chat feature unavailable", http.StatusServiceUnavailable, err.Error())
			return
		}
		handleResponse(c, h.log, "failed to delete session", http.StatusInternalServerError, err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

// ListChatMessages godoc
// @Router       /api/chat/sessions/{id}/messages [GET]
// @Summary      Get chat history for a session
// @Tags         chat
// @Param        id path string true "Session ID"
// @Success      200 {array} models.ChatMessage
func (h Handler) ListChatMessages(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		handleResponse(c, h.log, "missing session id", http.StatusBadRequest, "id is required")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	msgs, err := h.services.Chat().ListMessages(ctx, id)
	if err != nil {
		if errors.Is(err, service.ErrChatDisabled) {
			handleResponse(c, h.log, "chat feature unavailable", http.StatusServiceUnavailable, err.Error())
			return
		}
		handleResponse(c, h.log, "failed to list messages", http.StatusInternalServerError, err.Error())
		return
	}
	handleResponse(c, h.log, "messages fetched", http.StatusOK, msgs)
}

// SendChatMessage streams an assistant reply for a user message via SSE.
//
// The endpoint speaks the standard SSE wire format ("event: <type>\ndata:
// <json>\n\n") so the frontend can use the browser's EventSource API or a
// fetch + reader loop. Event types:
//
//   - meta:      first event, contains user_msg_id + retrieved chunks
//   - token:     incremental text deltas (many of these per request)
//   - citations: assembled list of [page N] references after the stream ends
//   - done:      terminal event with assistant_msg_id + token counts
//   - error:     terminal event with an error string (replaces "done")
//
// @Router /api/chat/sessions/{id}/messages [POST]
// @Tags chat
// @Param id path string true "Session ID"
// @Param request body models.ChatSendMessageRequest true "Message body"
// @Success 200 {string} string "text/event-stream"
func (h Handler) SendChatMessage(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		handleResponse(c, h.log, "missing session id", http.StatusBadRequest, "id is required")
		return
	}

	var req models.ChatSendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleResponse(c, h.log, "invalid request", http.StatusBadRequest, err.Error())
		return
	}
	if req.Content == "" && req.ImageBase64 == "" {
		handleResponse(c, h.log, "content or image required", http.StatusBadRequest, "send text, an image, or both")
		return
	}

	// Decode the optional inline image. We accept either a bare base64
	// blob or a "data:image/png;base64,..." data URL since browsers
	// produce both depending on how the file was read.
	var attachments []llm.Attachment
	if req.ImageBase64 != "" {
		raw := req.ImageBase64
		mime := req.ImageMIME
		if strings.HasPrefix(raw, "data:") {
			if comma := strings.Index(raw, ","); comma > 0 {
				header := raw[5:comma] // "image/png;base64"
				raw = raw[comma+1:]
				if semi := strings.Index(header, ";"); semi > 0 {
					if mime == "" {
						mime = header[:semi]
					}
				}
			}
		}
		bin, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			handleResponse(c, h.log, "invalid image_base64", http.StatusBadRequest, err.Error())
			return
		}
		// 10 MiB cap — bigger ones blow past Gemini's per-request budget
		// and slow the stream down dramatically.
		if len(bin) > 10*1024*1024 {
			handleResponse(c, h.log, "image too large (max 10 MiB)", http.StatusBadRequest, "")
			return
		}
		if mime == "" {
			mime = "image/jpeg" // sensible default
		}
		attachments = append(attachments, llm.Attachment{MIMEType: mime, Data: bin})
	}

	// Detach from gin's default request timeout — streaming responses can
	// run for ~30s and we don't want a stray middleware shortening that.
	ctx, cancel := context.WithTimeout(c.Request.Context(), 90*time.Second)
	defer cancel()

	stream, err := h.services.Chat().SendMessage(ctx, id, req.Content, attachments)
	if err != nil {
		if errors.Is(err, service.ErrChatDisabled) {
			handleResponse(c, h.log, "chat feature unavailable", http.StatusServiceUnavailable, err.Error())
			return
		}
		// 429 from Gemini: tell the client to retry instead of logging
		// the failure as a 500 (which spams the alert pipeline and
		// renders as "Internal Server Error" in the UI).
		if errors.Is(err, llm.ErrRateLimited) {
			handleResponse(c, h.log, "rate limit reached — please wait a moment and try again",
				http.StatusTooManyRequests, err.Error())
			return
		}
		handleResponse(c, h.log, "failed to start chat", http.StatusInternalServerError, err.Error())
		return
	}

	// SSE headers — disable nginx/proxy buffering so tokens flush in
	// real time. Caddy honours X-Accel-Buffering=no the same way nginx
	// does.
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(http.StatusOK)

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		// Should never happen with gin's default writer; bail with a
		// non-streaming error if it does.
		handleResponse(c, h.log, "streaming not supported", http.StatusInternalServerError, "")
		return
	}

	writeSSE := func(event string, payload any) error {
		b, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		if _, err := io.WriteString(c.Writer, fmt.Sprintf("event: %s\ndata: %s\n\n", event, b)); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}

	for ev := range stream {
		switch ev.Type {
		case "meta":
			if err := writeSSE("meta", gin.H{
				"user_msg_id": ev.UserMsgID,
				"retrieved": ev.Retrieved,
			}); err != nil {
				return // client disconnected
			}
		case "token":
			if err := writeSSE("token", gin.H{"text": ev.Delta}); err != nil {
				return
			}
		case "citations":
			if err := writeSSE("citations", gin.H{"citations": ev.Citations}); err != nil {
				return
			}
		case "done":
			_ = writeSSE("done", gin.H{
				"assistant_msg_id": ev.AssistantMsgID,
				"tokens_in":        ev.TokensIn,
				"tokens_out":       ev.TokensOut,
				"latency_ms":       ev.LatencyMs,
			})
		case "error":
			msg := ""
			if ev.Err != nil {
				msg = ev.Err.Error()
			}
			_ = writeSSE("error", gin.H{"error": msg})
		}
	}
}
