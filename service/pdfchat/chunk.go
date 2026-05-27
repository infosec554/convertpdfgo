package pdfchat

import (
	"strings"
	"unicode"
)

// Chunk is a unit we embed and retrieve. Page is the source page (1-indexed);
// Index is its position within that page (0-indexed) so two chunks from the
// same page can still be distinguished.
type Chunk struct {
	Page    int
	Index   int
	Content string
}

// Chunker tunables — keep small enough that 7 retrieved chunks fit in the
// LLM context budget alongside system prompt + chat history.
const (
	// chunkTargetSize is the soft size we aim for in characters. With
	// English-ish text this is ~150 tokens, so 7 chunks ≈ 1050 input
	// tokens for retrieval.
	chunkTargetSize = 700

	// chunkOverlap repeats the last N chars of each chunk at the start
	// of the next so a sentence split across the boundary still has a
	// chance of being retrieved fully.
	chunkOverlap = 120

	// minChunkSize avoids creating useless single-word chunks at page
	// ends. Anything shorter just gets merged into the previous one.
	minChunkSize = 80
)

// ChunkPages turns each Page into one or more Chunks. We chunk
// page-by-page rather than across pages because that lets us tag every
// chunk with its source page directly, no merge/split logic later.
//
// Pages with very little text (< minChunkSize) are emitted as a single
// chunk so they still appear in retrieval — page numbers are how
// citations stay honest.
func ChunkPages(pages []Page) []Chunk {
	out := make([]Chunk, 0, len(pages)*4)
	for _, p := range pages {
		text := strings.TrimSpace(p.Text)
		if text == "" {
			continue
		}
		for i, c := range chunkText(text) {
			out = append(out, Chunk{
				Page:    p.Number,
				Index:   i,
				Content: c,
			})
		}
	}
	return out
}

// chunkText splits a single page into overlapping chunks at sentence
// boundaries where possible. The algorithm:
//
//  1. Walk forward accumulating text until we cross chunkTargetSize.
//  2. From there, look back for the most recent sentence-ish break
//     (paragraph, sentence terminator, then any whitespace) and cut
//     there so we don't slice mid-word.
//  3. Start the next chunk chunkOverlap chars before the cut so the
//     context bleeds across boundaries.
//
// If a single "sentence" is somehow longer than chunkTargetSize (rare —
// usually tables in -layout mode), we fall back to a hard cut.
func chunkText(text string) []string {
	if len(text) <= chunkTargetSize {
		return []string{text}
	}

	var chunks []string
	runes := []rune(text)
	n := len(runes)
	start := 0

	for start < n {
		end := start + chunkTargetSize
		if end >= n {
			// Last bit — emit whatever's left. If it's tiny, merge it
			// into the previous chunk to avoid orphan fragments.
			tail := strings.TrimSpace(string(runes[start:]))
			if len(tail) < minChunkSize && len(chunks) > 0 {
				chunks[len(chunks)-1] = chunks[len(chunks)-1] + " " + tail
			} else if tail != "" {
				chunks = append(chunks, tail)
			}
			break
		}

		// Look backwards from `end` for a graceful break.
		cut := findBreak(runes, start+minChunkSize, end)
		if cut <= start {
			// No good break found — hard cut and move on.
			cut = end
		}

		piece := strings.TrimSpace(string(runes[start:cut]))
		if piece != "" {
			chunks = append(chunks, piece)
		}

		// Advance, leaving an overlap window so context bleeds across
		// boundaries. Cap overlap so we never go backwards more than we
		// went forward (defensive — happens with tiny `cut-start`).
		ov := chunkOverlap
		if ov > cut-start {
			ov = cut - start - 1
		}
		next := cut - ov
		if next <= start {
			next = cut // refuse to loop
		}
		start = next
	}

	return chunks
}

// findBreak scans runes[lo:hi] backwards for a good chunk boundary,
// preferring paragraph > sentence > word breaks. Returns the index AFTER
// the breaking character, or -1 if none found in the range.
func findBreak(runes []rune, lo, hi int) int {
	if hi > len(runes) {
		hi = len(runes)
	}
	if lo < 0 {
		lo = 0
	}
	// 1. Paragraph break (double newline).
	for i := hi - 1; i > lo; i-- {
		if runes[i] == '\n' && i > 0 && runes[i-1] == '\n' {
			return i + 1
		}
	}
	// 2. Sentence terminator followed by whitespace.
	for i := hi - 1; i > lo; i-- {
		switch runes[i] {
		case '.', '!', '?', '。', '！', '？':
			if i+1 < hi && unicode.IsSpace(runes[i+1]) {
				return i + 2
			}
		}
	}
	// 3. Any whitespace.
	for i := hi - 1; i > lo; i-- {
		if unicode.IsSpace(runes[i]) {
			return i + 1
		}
	}
	return -1
}
