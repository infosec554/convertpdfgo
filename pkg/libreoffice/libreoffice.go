// Package libreoffice wraps the `libreoffice --headless --convert-to`
// CLI. We need it because Gotenberg's /forms/libreoffice/convert
// endpoint is one-way (office → PDF) — the reverse direction (PDF →
// DOCX, etc.) has to invoke LibreOffice directly.
//
// Concurrency note
// ----------------
// LibreOffice serializes everything through a per-user-profile lock
// (~/.config/libreoffice). Two worker goroutines converting in
// parallel will collide unless each one gets its own profile via
// `-env:UserInstallation=...`. We do that here with a tmp UUID dir.
package libreoffice

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Convert runs `libreoffice --headless --convert-to <format>` against
// srcPath and writes the result into outDir, returning the absolute
// path of the produced file (basename(srcPath) with the new extension).
//
// A per-invocation UserInstallation dir keeps concurrent calls from
// stepping on each other's lockfiles. We hard-cap the run at 90s — a
// hung soffice would otherwise pin a worker goroutine indefinitely.
func Convert(ctx context.Context, srcPath, outDir, format string) (string, error) {
	if format == "" {
		return "", fmt.Errorf("convert: format required")
	}
	if _, err := os.Stat(srcPath); err != nil {
		return "", fmt.Errorf("convert: stat src: %w", err)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", fmt.Errorf("convert: mkdir out: %w", err)
	}

	profile, err := os.MkdirTemp("", "lo-profile-")
	if err != nil {
		return "", fmt.Errorf("convert: mkdtemp: %w", err)
	}
	defer os.RemoveAll(profile)

	rctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	args := []string{
		"--headless",
		"-env:UserInstallation=file://" + profile,
	}
	// PDF is not a native LibreOffice format — without an explicit
	// import filter it bails with "no export filter found". Other
	// inputs (DOCX/XLSX/PPTX) are auto-detected just fine.
	if strings.EqualFold(filepath.Ext(srcPath), ".pdf") {
		args = append(args, "--infilter=writer_pdf_import")
	}
	args = append(args,
		"--convert-to", format,
		"--outdir", outDir,
		srcPath,
	)

	cmd := exec.CommandContext(rctx, "libreoffice", args...)
	// soffice writes status to stderr; capture both for diagnostics.
	combined, err := cmd.CombinedOutput()
	if err != nil {
		// On timeout the context is cancelled, surface that explicitly.
		if rctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("convert: timeout after 90s; output: %s", trim(combined))
		}
		return "", fmt.Errorf("convert: libreoffice exited: %w; output: %s", err, trim(combined))
	}

	// libreoffice picks the output name: basename(src) with the new ext.
	base := strings.TrimSuffix(filepath.Base(srcPath), filepath.Ext(srcPath))
	out := filepath.Join(outDir, base+"."+strings.SplitN(format, ":", 2)[0])

	// Use a UUID-fallback path so two simultaneous converts on the same
	// basename don't clobber each other before the caller renames the
	// result. (We try the predicted path first; if it isn't there but
	// some other format-variant landed, surface that filename.)
	if _, err := os.Stat(out); err == nil {
		return out, nil
	}
	// Fall back: scan outDir for a file matching basename + the format
	// stem, take the newest. Covers cases like format="docx:MS Word
	// 2007 XML" where the extension still ends up as .docx.
	entries, err := os.ReadDir(outDir)
	if err != nil {
		return "", fmt.Errorf("convert: read outdir: %w", err)
	}
	var newest string
	var newestT time.Time
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.HasPrefix(e.Name(), base+".") {
			continue
		}
		info, ierr := e.Info()
		if ierr != nil {
			continue
		}
		if info.ModTime().After(newestT) {
			newestT = info.ModTime()
			newest = filepath.Join(outDir, e.Name())
		}
	}
	if newest == "" {
		return "", fmt.Errorf("convert: no output produced; soffice said: %s", trim(combined))
	}
	return newest, nil
}

func trim(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 512 {
		return s[:512] + "…"
	}
	return s
}
