package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/timeouts"
)

// ListAllFilesAdmin godoc
// @Router       /admin/files [get]
// @Security     ApiKeyAuth
// @Summary      List every file across users (admin)
// @Description  Pagination via limit/offset. Filter by user_id (or pass
//
//	"guest" for anonymous uploads), file_type extension, or
//	a free-text search on file_name.
//
// @Tags         admin-files
// @Param        user_id   query string false "user id, or 'guest' for anonymous"
// @Param        file_type query string false "file extension, e.g. .pdf"
// @Param        q         query string false "search in file_name (ILIKE)"
// @Param        limit     query int    false "default 50, max 500"
// @Param        offset    query int    false "default 0"
// @Success      200 {object} models.Response
func (h Handler) ListAllFilesAdmin(c *gin.Context) {
	var fl models.AdminFileFilter
	if err := c.ShouldBindQuery(&fl); err != nil {
		handleResponse(c, h.log, "invalid query", http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	items, total, err := h.services.File().ListAll(ctx, fl)
	if err != nil {
		handleResponse(c, h.log, "failed to list files", http.StatusInternalServerError, err.Error())
		return
	}
	handleResponse(c, h.log, "files listed", http.StatusOK, gin.H{
		"items":  items,
		"total":  total,
		"limit":  fl.Limit,
		"offset": fl.Offset,
	})
}

// DeleteFileAdmin godoc
// @Router       /admin/files/{id} [delete]
// @Security     ApiKeyAuth
// @Summary      Force-delete any file (admin)
// @Description  Removes the DB row + the on-disk blob. Useful when a user
//
//	can't reach their own /file/:id (e.g. they are blocked).
//
// @Tags         admin-files
// @Param        id path string true "file id"
// @Success      200 {object} models.Response
func (h Handler) DeleteFileAdmin(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		handleResponse(c, h.log, "missing file id", http.StatusBadRequest, nil)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	if err := h.services.File().Delete(ctx, id); err != nil {
		handleResponse(c, h.log, "failed to delete file", http.StatusInternalServerError, err.Error())
		return
	}
	h.audit(c, "admin_file_delete", c.GetString("user_id"), id, "")
	handleResponse(c, h.log, "file deleted", http.StatusOK, gin.H{"id": id})
}
