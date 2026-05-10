package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/pkg/timeouts"
)

// UploadFile godoc
// @Router       /file/upload [POST]
// @Security     ApiKeyAuth
// @Summary      Upload file
// @Tags         file
// @Accept       multipart/form-data
// @Produce      json
// @Param        file formData file true "Upload file"
// @Success      201  {object}  map[string]string
// @Failure      400  {object}  models.Response
// @Failure      500  {object}  models.Response
func (h Handler) UploadFile(c *gin.Context) {
	var ptrUserID *string
	if uid, exists := c.Get("user_id"); exists {
		if strID, ok := uid.(string); ok && strID != "" {
			ptrUserID = &strID
		}
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		handleResponse(c, h.log, "file is required", http.StatusBadRequest, err.Error())
		return
	}

	// Tiered upload caps:
	//   * Guests get 30 MB so most one-shot conversions just work.
	//   * Registered users get 100 MB. Anything over is a sign-in nudge,
	//     not a hard refusal — clients can show a "Register to upload
	//     larger files" CTA rather than a generic 400.
	//
	// The router-level BodyLimit middleware caps the request size
	// somewhat above this so the multipart parser doesn't waste memory.
	const (
		guestMaxSize      = 30 * 1024 * 1024  // 30 MB
		registeredMaxSize = 100 * 1024 * 1024 // 100 MB
	)
	if ptrUserID == nil && fileHeader.Size > guestMaxSize {
		handleResponse(c, h.log, "Register to upload files larger than 30 MB", http.StatusUnauthorized, gin.H{
			"limit_bytes":  guestMaxSize,
			"file_size":    fileHeader.Size,
			"sign_up_path": "/auth/register",
		})
		return
	}
	if ptrUserID != nil && fileHeader.Size > registeredMaxSize {
		handleResponse(c, h.log, "File exceeds your 100 MB account limit", http.StatusBadRequest, gin.H{
			"limit_bytes": registeredMaxSize,
			"file_size":   fileHeader.Size,
		})
		return
	}

	fileName := fileHeader.Filename
	fileType := filepath.Ext(fileName)
	fileID := uuid.NewString()
	savePath := fmt.Sprintf("uploads/%s%s", fileID, fileType)

	if err := os.MkdirAll("uploads", 0o755); err != nil {
		handleResponse(c, h.log, "failed to prepare uploads dir", http.StatusInternalServerError, err.Error())
		return
	}

	// Stream the upload to disk while computing SHA-256 in the same pass.
	src, err := fileHeader.Open()
	if err != nil {
		handleResponse(c, h.log, "failed to open upload", http.StatusInternalServerError, err.Error())
		return
	}
	defer src.Close()

	dst, err := os.Create(savePath)
	if err != nil {
		handleResponse(c, h.log, "failed to create file", http.StatusInternalServerError, err.Error())
		return
	}

	hasher := sha256.New()
	if _, err := io.Copy(io.MultiWriter(dst, hasher), src); err != nil {
		dst.Close()
		_ = os.Remove(savePath)
		handleResponse(c, h.log, "failed to save file", http.StatusInternalServerError, err.Error())
		return
	}
	if err := dst.Close(); err != nil {
		_ = os.Remove(savePath)
		handleResponse(c, h.log, "failed to flush file", http.StatusInternalServerError, err.Error())
		return
	}
	sha := hex.EncodeToString(hasher.Sum(nil))

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	// Deduplicate: if a file with this hash already exists, drop the
	// freshly written copy and reuse the previous row's ID.
	if existing, err := h.services.File().FindBySHA256(ctx, sha); err == nil && existing.ID != "" {
		_ = os.Remove(savePath)
		handleResponse(c, h.log, "file already exists (dedup)", http.StatusOK, gin.H{
			"id":   existing.ID,
			"dedup": true,
		})
		return
	} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		h.log.Error("dedup lookup failed", logger.Error(err))
	}

	file := models.File{
		ID:         fileID,
		UserID:     ptrUserID,
		FileName:   fileName,
		FilePath:   savePath,
		FileType:   fileType,
		FileSize:   fileHeader.Size,
		SHA256:     &sha,
		UploadedAt: time.Now(),
	}

	id, err := h.services.File().Upload(ctx, file)
	if err != nil {
		handleResponse(c, h.log, "upload failed", http.StatusInternalServerError, err.Error())
		return
	}

	handleResponse(c, h.log, "file uploaded", http.StatusCreated, gin.H{"id": id})
}

// GetFile godoc
// @Router       /file/{id} [GET]
// @Security     ApiKeyAuth
// @Summary      Get file by ID
// @Tags         file
// @Param        id path string true "File ID"
// @Produce      json
// @Success      200  {object}  models.File
// @Failure      404  {object}  models.Response
func (h Handler) GetFile(c *gin.Context) {
	id := c.Param("id")

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	file, err := h.services.File().Get(ctx, id)
	if err != nil {
		handleResponse(c, h.log, "file not found", http.StatusNotFound, err.Error())
		return
	}
	handleResponse(c, h.log, "file found", http.StatusOK, file)
}

// DeleteFile godoc
// @Router       /file/{id} [DELETE]
// @Security     ApiKeyAuth
// @Summary      Delete file by ID
// @Tags         file
// @Param        id path string true "File ID"
// @Success      200  {object}  map[string]string
// @Failure      500  {object}  models.Response
func (h Handler) DeleteFile(c *gin.Context) {
	id := c.Param("id")

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	if err := h.services.File().Delete(ctx, id); err != nil {
		handleResponse(c, h.log, "delete failed", http.StatusInternalServerError, err.Error())
		return
	}
	handleResponse(c, h.log, "file deleted", http.StatusOK, gin.H{"id": id})
}

// ListUserFiles godoc
// @Router       /file/list [GET]
// @Security     ApiKeyAuth
// @Summary      List all user's files
// @Tags         file
// @Produce      json
// @Success      200  {array}  models.File
// @Failure      500  {object}  models.Response
func (h Handler) ListUserFiles(c *gin.Context) {
	userID := c.GetString("user_id")

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	files, err := h.services.File().List(ctx, userID)
	if err != nil {
		handleResponse(c, h.log, "list failed", http.StatusInternalServerError, err.Error())
		return
	}
	handleResponse(c, h.log, "user files", http.StatusOK, files)
}

// CleanupOldFiles godoc
// @Summary      Cleanup old files
// @Description  Admin-only endpoint to delete files older than N days
// @Tags         file
// @Security     ApiKeyAuth
// @Produce      json
// @Success      200 {object} models.Response
// @Failure      500 {object} models.Response
// @Router       /api/files/cleanup [get]
func (h *Handler) CleanupOldFiles(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	const olderThanDays = 7

	count, err := h.services.File().CleanupOldFiles(ctx, olderThanDays)
	if err != nil {
		handleResponse(c, h.log, "failed to cleanup old files", http.StatusInternalServerError, err.Error())
		return
	}
	handleResponse(c, h.log, "old files cleaned up", http.StatusOK, gin.H{"deleted_files": count})
}
