package handler

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	timeouts "convertpdfgo/pkg/timeouts"
)

// PDFThumbnails godoc
// @Router  /api/pdf/thumbnails [post]
// @Summary Render each PDF page as a tiny preview image
// @Tags    pdf
// @Accept  json
// @Produce json
//
// Body: { file_id, max_pages?, max_width? }
// Returns: { count, page_count, pages: [{ n: 1, data: "<base64-jpg>" }, ...] }
//
// max_pages caps the response payload — clients that need a true page
// count for the full document still get it via the page_count field.
func (h Handler) PDFThumbnails(c *gin.Context) {
	var req struct {
		FileID   string `json:"file_id" binding:"required"`
		MaxPages int    `json:"max_pages"`
		MaxWidth int    `json:"max_width"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		handleResponse(c, h.log, "invalid request", http.StatusBadRequest, err.Error())
		return
	}
	if req.MaxPages <= 0 || req.MaxPages > 500 {
		req.MaxPages = 200
	}
	if req.MaxWidth <= 0 || req.MaxWidth > 400 {
		req.MaxWidth = 180
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeouts.DB)
	defer cancel()
	file, err := h.services.File().Get(ctx, req.FileID)
	if err != nil {
		handleResponse(c, h.log, "file not found", http.StatusNotFound, err.Error())
		return
	}
	if !strings.EqualFold(filepath.Ext(file.FileName), ".pdf") &&
		!strings.EqualFold(file.FileType, ".pdf") {
		handleResponse(c, h.log, "only PDFs have page thumbnails", http.StatusBadRequest, nil)
		return
	}

	// Real page count first — `pdfinfo` is in poppler-utils alongside pdftoppm.
	totalPages, _ := pdfPageCount(file.FilePath)

	tmpdir, err := os.MkdirTemp("", "pdfthumbs-*")
	if err != nil {
		handleResponse(c, h.log, "couldn't allocate temp", http.StatusInternalServerError, err.Error())
		return
	}
	defer os.RemoveAll(tmpdir)

	last := req.MaxPages
	if totalPages > 0 && totalPages < last {
		last = totalPages
	}

	// pdftoppm -jpeg -scale-to-x N -scale-to-y -1 input prefix
	// -l N caps at page N. We render at the requested width with
	// proportional height (-1).
	cmdCtx, cancelCmd := context.WithTimeout(c.Request.Context(), 25*time.Second)
	defer cancelCmd()
	prefix := filepath.Join(tmpdir, "p")
	cmd := exec.CommandContext(cmdCtx,
		"pdftoppm",
		"-jpeg",
		"-jpegopt", "quality=70",
		"-scale-to-x", strconv.Itoa(req.MaxWidth),
		"-scale-to-y", "-1",
		"-l", strconv.Itoa(last),
		file.FilePath,
		prefix,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		handleResponse(c, h.log, "thumbnail render failed", http.StatusInternalServerError,
			fmt.Sprintf("pdftoppm: %s: %v", strings.TrimSpace(string(out)), err))
		return
	}

	entries, err := os.ReadDir(tmpdir)
	if err != nil {
		handleResponse(c, h.log, "couldn't read thumbnails", http.StatusInternalServerError, err.Error())
		return
	}

	type page struct {
		N    int    `json:"n"`
		Data string `json:"data"` // raw base64 (no data: prefix — keeps payload smaller)
	}
	pages := make([]page, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jpg") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(tmpdir, e.Name()))
		if err != nil {
			continue
		}
		n := parsePdftoppmPageNum(e.Name())
		pages = append(pages, page{
			N:    n,
			Data: base64.StdEncoding.EncodeToString(raw),
		})
	}
	sort.Slice(pages, func(i, j int) bool { return pages[i].N < pages[j].N })

	handleResponse(c, h.log, "thumbnails", http.StatusOK, gin.H{
		"count":      len(pages),
		"page_count": totalPages,
		"pages":      pages,
	})
}

// parsePdftoppmPageNum picks the trailing integer out of names like
// "p-1.jpg" / "p-001.jpg" / "p-12.jpg".
func parsePdftoppmPageNum(name string) int {
	base := strings.TrimSuffix(filepath.Base(name), ".jpg")
	if i := strings.LastIndexByte(base, '-'); i >= 0 && i < len(base)-1 {
		if n, err := strconv.Atoi(strings.TrimLeft(base[i+1:], "0")); err == nil {
			return n
		}
		// All-zero suffix is page 0 / file-naming quirk; fall back to 1.
		if base[i+1:] == "" || strings.Trim(base[i+1:], "0") == "" {
			return 1
		}
	}
	return 0
}

// pdfPageCount shells out to pdfinfo for a cheap, accurate count.
// Returns 0 if anything fails — callers fall back to len(rendered).
func pdfPageCount(path string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "pdfinfo", path).Output()
	if err != nil {
		return 0, err
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "Pages:") {
			n, _ := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "Pages:")))
			return n, nil
		}
	}
	return 0, nil
}
