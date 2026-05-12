// Package ocr wraps the OCR pipeline our /api/pdf/ocr* endpoints lean
// on. Two binaries are involved:
//
//   - ocrmypdf — adds a searchable text layer to a PDF (the
//     "headline" OCR use case). Wraps tesseract internally and adds
//     PDF-specific niceties (page rasterisation, lossy/lossless
//     re-encoding, /OCRProperties metadata).
//   - tesseract  — used directly for the simpler single-image and
//     analytic paths (image→text, language detection, hOCR-style
//     structured output) where ocrmypdf would be overkill.
//
// Both are shelled out via exec.CommandContext so cancellation /
// timeout from the worker propagates correctly.
package ocr

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Options configures a regular `ocrmypdf` invocation (PDF in, PDF out
// with the text layer). Zero values yield ocrmypdf's safe defaults.
type Options struct {
	Languages string // e.g. "eng" or "eng+uzb+rus"; empty → "eng"
	ForceOCR  bool   // re-OCR pages that already have a text layer
	SkipText  bool   // pass pages through untouched when they already have text
	Sidecar   string // if non-empty, write the OCR'd plain text here
	Deskew    bool   // straighten scanned pages
	OutputPDFA bool  // emit PDF/A instead of PDF
}

// OCR runs ocrmypdf src → dst with the given options. The 5-min cap
// is generous for ~50-page docs on a single-CPU runner.
func OCR(ctx context.Context, srcPath, dstPath string, opts Options) error {
	if _, err := os.Stat(srcPath); err != nil {
		return fmt.Errorf("ocr: stat src: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return fmt.Errorf("ocr: mkdir out: %w", err)
	}
	if opts.Languages == "" {
		opts.Languages = "eng"
	}

	rctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	args := []string{"-l", opts.Languages}
	if opts.ForceOCR {
		args = append(args, "--force-ocr")
	} else if opts.SkipText {
		args = append(args, "--skip-text")
	}
	if opts.Deskew {
		args = append(args, "--deskew")
	}
	if opts.OutputPDFA {
		args = append(args, "--output-type", "pdfa")
	}
	if opts.Sidecar != "" {
		args = append(args, "--sidecar", opts.Sidecar)
	}
	args = append(args, srcPath, dstPath)

	cmd := exec.CommandContext(rctx, "ocrmypdf", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if rctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("ocr: timeout after 5m; output: %s", trim(out))
		}
		return fmt.Errorf("ocr: ocrmypdf exited: %w; output: %s", err, trim(out))
	}
	return nil
}

// TextFromImage runs `tesseract <src> stdout` on a single image and
// returns the recognized text. Used by /api/pdf/image-to-text.
func TextFromImage(ctx context.Context, srcPath, lang string) (string, error) {
	if lang == "" {
		lang = "eng"
	}
	rctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	cmd := exec.CommandContext(rctx, "tesseract", srcPath, "stdout", "-l", lang)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("tesseract: %w; stderr: %s", err, trim(stderr.Bytes()))
	}
	return stdout.String(), nil
}

// DetectLanguage runs `tesseract <src> stdout --psm 0` (OSD —
// orientation & script detection) and parses the Script/Confidence
// it reports. PSM=0 doesn't recognise text; it only identifies the
// script (Latin / Cyrillic / Arabic / Han / …). We map back to a
// tesseract language code that's likely to recognise that script,
// since the caller usually wants a code they can plug into OCR().
//
// Returns (lang_code, confidence_0..100). Empty lang means the
// detection failed even though tesseract didn't error.
func DetectLanguage(ctx context.Context, srcPath string) (string, float64, error) {
	rctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(rctx, "tesseract", srcPath, "-", "--psm", "0")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", 0, fmt.Errorf("tesseract osd: %w; stderr: %s", err, trim(stderr.Bytes()))
	}
	// OSD output goes to stderr in some builds, stdout in others;
	// merge before parsing.
	osd := stdout.String() + "\n" + stderr.String()
	return parseOSD(osd)
}

// osdScript captures the "Script: Latin" / "Script confidence: 12.34"
// lines tesseract --psm 0 emits.
var (
	scriptRE     = regexp.MustCompile(`(?m)^Script:\s*(\S+)`)
	confidenceRE = regexp.MustCompile(`(?m)^Script confidence:\s*([0-9.]+)`)
)

func parseOSD(osd string) (string, float64, error) {
	scriptMatch := scriptRE.FindStringSubmatch(osd)
	confMatch := confidenceRE.FindStringSubmatch(osd)
	if scriptMatch == nil {
		return "", 0, fmt.Errorf("osd: no Script line in: %s", trim([]byte(osd)))
	}
	script := scriptMatch[1]
	var conf float64
	if confMatch != nil {
		conf, _ = strconv.ParseFloat(confMatch[1], 64)
	}
	return scriptToLang(script), conf, nil
}

// scriptToLang picks a sensible tesseract language code that can
// recognise text in the given script. Coarse but useful — the
// caller can override if they know the real language.
func scriptToLang(script string) string {
	switch strings.ToLower(script) {
	case "latin":
		return "eng"
	case "cyrillic":
		return "rus"
	case "arabic":
		return "ara"
	case "han":
		return "chi_sim"
	case "japanese":
		return "jpn"
	case "korean":
		return "kor"
	default:
		// Unknown script — return as-is so callers can still log it.
		return strings.ToLower(script)
	}
}

// HOCR runs tesseract with the hocr config to produce HTML/XML-ish
// output that includes per-word bounding boxes. The wrapper writes
// to dstPath (.hocr extension expected) and returns nil on success.
// /api/pdf/ocr-structured serves this file directly.
func HOCR(ctx context.Context, srcPath, dstPath, lang string) error {
	if lang == "" {
		lang = "eng"
	}
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return fmt.Errorf("hocr: mkdir: %w", err)
	}
	// Tesseract writes <stem>.hocr — strip the extension off dstPath
	// for the CLI arg, then verify the expected file exists.
	stem := strings.TrimSuffix(dstPath, filepath.Ext(dstPath))

	rctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(rctx, "tesseract", srcPath, stem, "-l", lang, "hocr")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("hocr: tesseract: %w; output: %s", err, trim(out))
	}
	if _, err := os.Stat(stem + ".hocr"); err != nil {
		return fmt.Errorf("hocr: expected %s.hocr; got: %s", stem, trim(out))
	}
	if stem+".hocr" != dstPath {
		if err := os.Rename(stem+".hocr", dstPath); err != nil {
			return fmt.Errorf("hocr: rename: %w", err)
		}
	}
	return nil
}

func trim(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 512 {
		return s[:512] + "…"
	}
	return s
}
