// Contact-form handlers.
//
//   - POST /api/contact            (public; rate-limited globally)
//   - GET  /admin/contacts          (admin, contacts:read)
//   - GET  /admin/contacts/:id      (admin, contacts:read)
//   - POST /admin/contacts/:id/reply (admin, contacts:write; emails user)
//   - PUT  /admin/contacts/:id/status (admin, contacts:write)
//   - DELETE /admin/contacts/:id    (admin, contacts:write)
package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/timeouts"
)

// --- User-facing -------------------------------------------------------

// SubmitContact godoc
// @Summary  Submit a contact-form message to the admins
// @Tags     contact
// @Accept   json
// @Produce  json
// @Param    request body models.ContactSubmitRequest true "contact"
// @Success  201 {object} map[string]string
// @Router   /api/contact [post]
func (h Handler) SubmitContact(c *gin.Context) {
	var req models.ContactSubmitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleResponse(c, h.log, "invalid contact request", http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	id, err := h.services.Contact().Submit(ctx, req, userPtr(c))
	if err != nil {
		handleResponse(c, h.log, "submit failed", http.StatusInternalServerError, err.Error())
		return
	}
	handleResponse(c, h.log, "received", http.StatusCreated, gin.H{"id": id})
}

// --- Admin-facing ------------------------------------------------------

// ListContactsAdmin godoc
// @Summary  List contact messages with filters + pagination
// @Tags     admin-contact
// @Produce  json
// @Param    status query string false "new | replied | closed"
// @Param    q      query string false "ILIKE name/email/subject"
// @Param    limit  query int    false "default 50, max 200"
// @Param    offset query int    false "default 0"
// @Success  200 {object} map[string]interface{}
// @Router   /admin/contacts [get]
// @Security ApiKeyAuth
func (h Handler) ListContactsAdmin(c *gin.Context) {
	var f models.ContactListFilter
	_ = c.ShouldBindQuery(&f)

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()
	rows, total, err := h.services.Contact().List(ctx, f)
	if err != nil {
		handleResponse(c, h.log, "list failed", http.StatusInternalServerError, err.Error())
		return
	}
	handleResponse(c, h.log, "ok", http.StatusOK, gin.H{
		"items": rows,
		"total": total,
	})
}

// GetContactAdmin godoc
// @Summary  Fetch one contact message + its full reply thread
// @Tags     admin-contact
// @Produce  json
// @Param    id path string true "message id"
// @Success  200 {object} models.ContactMessage
// @Router   /admin/contacts/{id} [get]
// @Security ApiKeyAuth
func (h Handler) GetContactAdmin(c *gin.Context) {
	id := c.Param("id")
	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()
	m, err := h.services.Contact().Get(ctx, id)
	if err != nil {
		handleResponse(c, h.log, "not found", http.StatusNotFound, err.Error())
		return
	}
	handleResponse(c, h.log, "ok", http.StatusOK, m)
}

// ReplyContactAdmin godoc
// @Summary  Reply to a contact message — saves the reply + emails the sender
// @Tags     admin-contact
// @Accept   json
// @Produce  json
// @Param    id path string true "message id"
// @Param    request body models.ContactReplyRequest true "reply"
// @Success  201 {object} map[string]string
// @Router   /admin/contacts/{id}/reply [post]
// @Security ApiKeyAuth
func (h Handler) ReplyContactAdmin(c *gin.Context) {
	id := c.Param("id")
	var req models.ContactReplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleResponse(c, h.log, "invalid reply", http.StatusBadRequest, err.Error())
		return
	}
	adminID := c.GetString("user_id")
	if adminID == "" {
		// Shouldn't happen — AuthorizerMiddleware sets it — but be
		// explicit so a future routing edit can't silently let an
		// anonymous reply through.
		handleResponse(c, h.log, "auth required", http.StatusUnauthorized, nil)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()
	replyID, err := h.services.Contact().Reply(ctx, id, adminID, req)
	if err != nil {
		handleResponse(c, h.log, "reply failed", http.StatusBadRequest, err.Error())
		return
	}
	handleResponse(c, h.log, "replied", http.StatusCreated, gin.H{
		"reply_id": replyID,
		"status":   "replied",
	})
}

// UpdateContactStatusAdmin godoc
// @Summary  Manually change a contact message's status (no email)
// @Tags     admin-contact
// @Accept   json
// @Produce  json
// @Param    id path string true "message id"
// @Param    request body models.ContactUpdateStatusRequest true "status"
// @Success  200 {object} map[string]string
// @Router   /admin/contacts/{id}/status [put]
// @Security ApiKeyAuth
func (h Handler) UpdateContactStatusAdmin(c *gin.Context) {
	id := c.Param("id")
	var req models.ContactUpdateStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleResponse(c, h.log, "invalid status", http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()
	if err := h.services.Contact().UpdateStatus(ctx, id, req.Status); err != nil {
		handleResponse(c, h.log, "update failed", http.StatusInternalServerError, err.Error())
		return
	}
	handleResponse(c, h.log, "ok", http.StatusOK, gin.H{"id": id, "status": req.Status})
}

// DeleteContactAdmin godoc
// @Summary  Hard-delete a contact message (cascades replies)
// @Tags     admin-contact
// @Produce  json
// @Param    id path string true "message id"
// @Success  200 {object} map[string]string
// @Router   /admin/contacts/{id} [delete]
// @Security ApiKeyAuth
func (h Handler) DeleteContactAdmin(c *gin.Context) {
	id := c.Param("id")
	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()
	if err := h.services.Contact().Delete(ctx, id); err != nil {
		handleResponse(c, h.log, "delete failed", http.StatusInternalServerError, err.Error())
		return
	}
	handleResponse(c, h.log, "deleted", http.StatusOK, gin.H{"id": id})
}
