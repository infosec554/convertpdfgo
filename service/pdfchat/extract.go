// Package pdfchat builds the indexing + retrieval pipeline behind the
// /chat tool. extract.go pulls text out of a PDF page-by-page so we can
// preserve page numbers all the way into the citations the LLM emits.
package pdfchat

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Page holds one extracted page of a PDF. Number is 1-indexed so it
// matches what a human reader would expect to see in a citation badge.
type Page struct {
	Number int
	Text   string
}

// ExtractPages shells out to pdftotext (poppler-utils — same binary the
// /extract-text tool already uses) and splits the result on 0x0C form-feed
// characters. pdftotext writes one FF between every two pages, which is
// the standard the Unix PDF tooling has agreed on for decades.
//
// Pages with no extractable text (scanned images, etc.) come back with
// Text == "" — callers can decide whether to OCR them or skip. We do NOT
// silently drop empty pages because the page numbering needs to stay
// continuous for citations to be meaningful.
//
// Pass a context with timeout; pdftotext can hang on malformed PDFs.
func ExtractPages(ctx context.Context, pdfPath string) ([]Page, error) {
	if _, err := os.Stat(pdfPath); err != nil {
		return nil, fmt.Errorf("pdfchat extract: stat input: %w", err)
	}

	// `-` writes to stdout, which avoids the temp-file dance.
	// `-layout` keeps reading order useful for prose-heavy PDFs (it tries
	// to preserve columns), at a small cost vs the default mode for
	// dense tables. For chat indexing we want prose readability, so
	// layout mode wins.
	cmd := exec.CommandContext(ctx, "pdftotext", "-layout", "-enc", "UTF-8", pdfPath, "-")
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("pdfchat extract: pdftotext exit %d: %s",
				ee.ExitCode(), strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, fmt.Errorf("pdfchat extract: pdftotext: %w", err)
	}

	// Form-feed (0x0C) separates pages in pdftotext output.
	rawPages := strings.Split(string(out), "\f")

	// pdftotext appends a trailing empty page after the last form-feed.
	// Trim it so PageCount == real page count.
	if n := len(rawPages); n > 0 && strings.TrimSpace(rawPages[n-1]) == "" {
		rawPages = rawPages[:n-1]
	}

	pages := make([]Page, len(rawPages))
	for i, t := range rawPages {
		pages[i] = Page{
			Number: i + 1,
			Text:   normaliseWhitespace(t),
		}
	}
	return pages, nil
}

// normaliseWhitespace collapses runs of blank lines (pdftotext leaves a
// lot of those with -layout) and strips trailing space on each line so
// chunk boundaries don't get pushed around by visual padding.
func normaliseWhitespace(s string) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	blank := 0
	for _, ln := range lines {
		ln = strings.TrimRight(ln, " \t")
		if ln == "" {
			blank++
			if blank > 1 {
				continue // collapse 3+ blank lines into one
			}
		} else {
			blank = 0
		}
		out = append(out, ln)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}
