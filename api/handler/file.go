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

	// Tiered upload caps (tightened from earlier 30/100 MB):
	//   * Guests get 20 MB — most invoice / receipt scans still fit.
	//   * Registered users get 30 MB. Anything over is a sign-in nudge
	//     for guests (clients can show a "Register to upload larger
	//     files" CTA) and a hard refusal for members.
	//
	// The router-level BodyLimit middleware caps the request size
	// somewhat above this so the multipart parser doesn't waste memory.
	const (
		guestMaxSize      = 20 * 1024 * 1024 // 20 MB
		registeredMaxSize = 30 * 1024 * 1024 // 30 MB
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
			"id":    existing.ID,
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
// @Summary      Get file by ID (guest-owned: open; user-owned: owner or admin)
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
	// Same ownership gate as DownloadFile: guest uploads (UserID nil)
	// are visible to anyone who knows the ID; owned files require the
	// owner or an admin. 404 (not 403) so the response is identical
	// whether the file exists or just isn't yours.
	if !canAccessFile(c, file.UserID) {
		handleResponse(c, h.log, "file not found", http.StatusNotFound, "")
		return
	}
	handleResponse(c, h.log, "file found", http.StatusOK, file)
}

// DownloadFile streams the binary content of a stored file.
//
// Authorization model:
//
//   - Guest files (user_id IS NULL): anyone with the file ID can download.
//     The ID is a UUIDv4 — unguessable in practice.
//   - Owned files (user_id IS NOT NULL): only the owner or an admin
//     can download. Other guests / users hit 404 (the same code as
//     "file not found" so the endpoint doesn't reveal whether a given
//     ID exists).
//
// OptionalAuthMiddleware populates user_id when a valid Bearer token is
// attached; absence means the caller is treated as a guest.
//
// @Router       /file/{id}/download [GET]
// @Summary      Download raw file bytes
// @Tags         file
// @Param        id path string true "File ID"
// @Success      200 {file} binary
// @Failure      404 {object} models.Response
func (h Handler) DownloadFile(c *gin.Context) {
	id := c.Param("id")

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	file, err := h.services.File().Get(ctx, id)
	if err != nil {
		handleResponse(c, h.log, "file not found", http.StatusNotFound, err.Error())
		return
	}

	// Ownership gate, shared with GetFile/DeleteFile via canAccessFile.
	// Use 404 (not 403) so the response is identical whether the file
	// exists or not — no probing-via-error-code.
	if !canAccessFile(c, file.UserID) {
		handleResponse(c, h.log, "file not found", http.StatusNotFound, "no such file")
		return
	}

	if _, err := os.Stat(file.FilePath); err != nil {
		handleResponse(c, h.log, "file gone from disk", http.StatusNotFound, "the file was already cleaned up")
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, file.FileName))
	c.Header("Content-Type", file.FileType)
	c.File(file.FilePath)
}

// DeleteFile godoc
// @Router       /file/{id} [DELETE]
// @Summary      Delete file by ID (guest-owned: open; user-owned: owner or admin)
// @Tags         file
// @Param        id path string true "File ID"
// @Success      200  {object}  map[string]string
// @Failure      500  {object}  models.Response
func (h Handler) DeleteFile(c *gin.Context) {
	id := c.Param("id")

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()

	// Look up first so the ownership check runs *before* we touch
	// any state. Same 404-for-not-yours pattern as GetFile.
	file, err := h.services.File().Get(ctx, id)
	if err != nil {
		handleResponse(c, h.log, "file not found", http.StatusNotFound, err.Error())
		return
	}
	if !canAccessFile(c, file.UserID) {
		handleResponse(c, h.log, "file not found", http.StatusNotFound, "")
		return
	}
	if err := h.services.File().Delete(ctx, id); err != nil {
		handleResponse(c, h.log, "delete failed", http.StatusInternalServerError, err.Error())
		return
	}
	handleResponse(c, h.log, "file deleted", http.StatusOK, gin.H{"id": id})
}

// canAccessFile is the shared file-ownership gate behind GetFile,
// DeleteFile, DownloadFile. A guest file (UserID nil/"") is open to
// anyone who knows the ID; an owned file requires the caller to BE
// the owner or to hold the admin role.
func canAccessFile(c *gin.Context, ownerID *string) bool {
	if ownerID == nil || *ownerID == "" {
		return true
	}
	if c.GetString("user_role") == "admin" {
		return true
	}
	return c.GetString("user_id") == *ownerID
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
