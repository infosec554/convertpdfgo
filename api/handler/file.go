package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/bufpool"
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

	// Tiered upload caps:
	//   * Guests get 30 MB — most everyday invoices / scans / reports fit.
	//   * Registered users get 50 MB. Larger than 30 MB as a guest gets a
	//     401 with sign_up_path so the UI can render a "Sign in with
	//     Google / GitHub to upload larger files" CTA. Larger than 50 MB
	//     for a signed-in user is a hard refusal.
	//
	// The router-level BodyLimit middleware caps the request size at
	// ~110 MiB so the multipart parser doesn't waste memory on rejects.
	const (
		guestMaxSize      = 30 * 1024 * 1024 // 30 MB
		registeredMaxSize = 50 * 1024 * 1024 // 50 MB
	)
	if ptrUserID == nil && fileHeader.Size > guestMaxSize {
		// Email+password registration was removed — point the UI at the
		// OAuth providers we actually support.
		handleResponse(c, h.log, "Sign in to upload files larger than 30 MB", http.StatusUnauthorized, gin.H{
			"limit_bytes":   guestMaxSize,
			"file_size":     fileHeader.Size,
			"sign_in_paths": []string{"/auth/google", "/auth/github"},
		})
		return
	}
	if ptrUserID != nil && fileHeader.Size > registeredMaxSize {
		handleResponse(c, h.log, "File exceeds your 50 MB account limit", http.StatusBadRequest, gin.H{
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

	// 1 MiB CopyBuffer keeps syscall count low for big uploads — Go's
	// default 32 KiB io.Copy does ~30× more read/write calls per file.
	// bufpool reuses the slab across requests so this isn't an alloc.
	hasher := sha256.New()
	copyBuf := bufpool.GetBytes()
	_, err = io.CopyBuffer(io.MultiWriter(dst, hasher), src, copyBuf)
	bufpool.PutBytes(copyBuf)
	if err != nil {
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

	// Deduplicate within the caller's own slice. Cross-user dedup would
	// silently expose private uploads — a guest pre-populating the
	// content hash and then any signed-in user reuploading the same
	// bytes would have inherited the orphan row. Scoping the lookup to
	// `(sha256, user_id)` prevents that.
	if existing, err := h.services.File().FindBySHA256(ctx, sha, ptrUserID); err == nil && existing.ID != "" {
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

	fi, err := os.Stat(file.FilePath)
	if err != nil {
		handleResponse(c, h.log, "file gone from disk", http.StatusNotFound, "the file was already cleaned up")
		return
	}

	// Legacy uploads stored the file extension (e.g. ".pdf") in FileType
	// instead of a real MIME. Map back to the proper Content-Type so
	// browsers can preview PDFs/images inline instead of treating them
	// as text/octet-streams. Fall back to the stored value when it
	// already looks like a MIME ("type/subtype"), and to a safe default
	// for genuinely unknown extensions.
	contentType := resolveContentType(file.FileType, file.FileName)
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, file.FileName))
	c.Header("Content-Type", contentType)
	c.Header("Accept-Ranges", "bytes")

	// ETag enables If-None-Match → 304 from http.ServeContent. The sha256
	// is the file's content hash, so it's a perfect, stable etag. Files
	// without sha256 (legacy rows) fall back to no etag — still served
	// fine, just with no cache validation.
	if file.SHA256 != nil && *file.SHA256 != "" {
		c.Header("ETag", `"`+*file.SHA256+`"`)
		// Re-validate every hour: user-private content, cache OK but
		// must check sha didn't change before reusing the body.
		c.Header("Cache-Control", "private, max-age=3600, must-revalidate")
	}

	// ServeContent honours Range, If-None-Match, If-Modified-Since,
	// HEAD — so a partial download (e.g. browser preview seeking the
	// last 1 KiB of a large PDF) costs Range bytes instead of a full
	// re-transfer. c.File previously bypassed Range with an extra Stat
	// + io.Copy of the whole file.
	f, err := os.Open(file.FilePath)
	if err != nil {
		handleResponse(c, h.log, "failed to open file", http.StatusInternalServerError, err.Error())
		return
	}
	defer f.Close()
	http.ServeContent(c.Writer, c.Request, file.FileName, fi.ModTime(), f)
}

// resolveContentType normalises the heterogeneous FileType column into a
// real MIME string. The upload handler used to stash filepath.Ext() in
// here (".pdf", ".jpg"), so older rows have an extension; newer rows
// might carry a proper "application/pdf" already. We accept either.
//
// The fileName fallback covers cases where FileType is blank or junk —
// we sniff the extension from the saved filename. octet-stream is the
// last-resort default so the response is still a valid HTTP message.
func resolveContentType(fileType, fileName string) string {
	if strings.Contains(fileType, "/") {
		return fileType // already a MIME
	}
	ext := fileType
	if !strings.HasPrefix(ext, ".") {
		ext = filepath.Ext(fileName)
	}
	if ext == "" {
		return "application/octet-stream"
	}
	if mt := mime.TypeByExtension(ext); mt != "" {
		return mt
	}
	return "application/octet-stream"
}

// ImportFile pulls a file from an external URL (Dropbox Chooser direct
// link, Google Drive download URL, plain https) and registers it in our
// files table as if the user had uploaded it locally. The frontend's
// "Dropbox" / "Google Drive" buttons hit this endpoint with the URL the
// provider's picker handed back, so the rest of the pipeline doesn't
// have to know where the file came from.
//
// Body: { provider: "dropbox"|"gdrive"|"url", url: "...", name?: "..." }
//
// Auth: OptionalAuth — guests can import too (same tier caps as upload).
// Size: caps mirror /file/upload (30 MB guest, 50 MB signed-in).
//
// @Router  /file/import [POST]
// @Tags    file
func (h Handler) ImportFile(c *gin.Context) {
	var req struct {
		Provider string `json:"provider"`
		URL      string `json:"url" binding:"required"`
		Name     string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		handleResponse(c, h.log, "invalid request", http.StatusBadRequest, err.Error())
		return
	}
	// Allow only https URLs — http would let an attacker downgrade and
	// localhost would let them reach internal services (SSRF).
	if !strings.HasPrefix(req.URL, "https://") {
		handleResponse(c, h.log, "https url required", http.StatusBadRequest, "url must be https")
		return
	}

	var ptrUserID *string
	if uid, exists := c.Get("user_id"); exists {
		if strID, ok := uid.(string); ok && strID != "" {
			ptrUserID = &strID
		}
	}
	const (
		guestMaxSize      = 30 * 1024 * 1024
		registeredMaxSize = 50 * 1024 * 1024
	)
	maxSize := int64(guestMaxSize)
	if ptrUserID != nil {
		maxSize = int64(registeredMaxSize)
	}

	// Fetch with a generous timeout — Dropbox direct links can be slow
	// on first hit while their CDN warms up.
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, req.URL, nil)
	if err != nil {
		handleResponse(c, h.log, "build request failed", http.StatusBadRequest, err.Error())
		return
	}
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		handleResponse(c, h.log, "fetch from "+req.Provider+" failed", http.StatusBadGateway, err.Error())
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		handleResponse(c, h.log, "cloud download non-OK", http.StatusBadGateway,
			fmt.Sprintf("upstream returned HTTP %d", resp.StatusCode))
		return
	}

	// Pick a filename: prefer the caller's hint, then the Content-
	// Disposition header from the provider, then the URL path's last
	// segment.
	fileName := req.Name
	if fileName == "" {
		if cd := resp.Header.Get("Content-Disposition"); cd != "" {
			if _, params, err := mime.ParseMediaType(cd); err == nil {
				fileName = params["filename"]
			}
		}
	}
	if fileName == "" {
		// last URL path segment, sans query
		bare := req.URL
		if i := strings.IndexByte(bare, '?'); i > 0 {
			bare = bare[:i]
		}
		if i := strings.LastIndexByte(bare, '/'); i >= 0 && i < len(bare)-1 {
			fileName = bare[i+1:]
		}
	}
	if fileName == "" {
		fileName = "imported"
	}

	fileID := uuid.NewString()
	ext := filepath.Ext(fileName)
	savePath := fmt.Sprintf("uploads/%s%s", fileID, ext)
	if err := os.MkdirAll("uploads", 0o755); err != nil {
		handleResponse(c, h.log, "prepare uploads dir failed", http.StatusInternalServerError, err.Error())
		return
	}

	dst, err := os.Create(savePath)
	if err != nil {
		handleResponse(c, h.log, "create file failed", http.StatusInternalServerError, err.Error())
		return
	}
	hash := sha256.New()
	// Cap how much we copy at maxSize+1 so an oversized payload trips
	// the limit instead of filling the disk.
	limited := &io.LimitedReader{R: resp.Body, N: maxSize + 1}
	written, err := io.Copy(io.MultiWriter(dst, hash), limited)
	dst.Close()
	if err != nil {
		os.Remove(savePath)
		handleResponse(c, h.log, "download stream failed", http.StatusBadGateway, err.Error())
		return
	}
	if written > maxSize {
		os.Remove(savePath)
		handleResponse(c, h.log,
			fmt.Sprintf("file too large (%d > %d bytes); sign in for the higher tier", written, maxSize),
			http.StatusRequestEntityTooLarge, nil)
		return
	}

	sha := hex.EncodeToString(hash.Sum(nil))

	// Use mime detection so Content-Type stays sensible on download.
	fileType := resp.Header.Get("Content-Type")
	if fileType == "" || strings.HasPrefix(fileType, "application/octet-stream") {
		if mt := mime.TypeByExtension(ext); mt != "" {
			fileType = mt
		} else {
			fileType = "application/octet-stream"
		}
	}

	ctxDB, cancelDB := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancelDB()

	savedID, err := h.services.File().Upload(ctxDB, models.File{
		ID:         fileID,
		UserID:     ptrUserID,
		FileName:   fileName,
		FilePath:   savePath,
		FileType:   fileType,
		FileSize:   written,
		UploadedAt: time.Now(),
		SHA256:     &sha,
	})
	if err != nil {
		os.Remove(savePath)
		handleResponse(c, h.log, "save failed", http.StatusInternalServerError, err.Error())
		return
	}

	h.log.Info("file imported", logger.String("provider", req.Provider),
		logger.String("id", savedID), logger.Any("size", written))
	handleResponse(c, h.log, "file imported", http.StatusCreated, gin.H{
		"id": savedID,
	})
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
