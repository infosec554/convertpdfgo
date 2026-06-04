package pdfchat

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"convertpdfgo/api/models"
)

// BuildPrompt assembles the system prompt + retrieved-chunk context that
// goes alongside the user's question + recent history. The function
// returns the System string and the Messages slice the LLM expects.
//
// We separate system + messages because Gemini wants `systemInstruction`
// in its own field — the llm.Message{Role: System} path also works but
// keeping the structure explicit makes the prompt easier to reason about.
func BuildPrompt(retrieved []models.PDFChunkWithScore, history []models.ChatMessage, question string) (string, []ChatTurn) {
	system := `You are a helpful assistant that answers questions based on the provided PDF document excerpts below.

Rules you MUST follow:
1. For SPECIFIC factual questions ("when did X happen", "what is Y's salary"), every claim must be supported by the excerpts. If the answer truly is not in them, reply in the user's language with "I couldn't find that in the document."
2. For GENERAL/OVERVIEW questions ("what is this document about", "summarize", "main topics", "what does this contain"), produce a 2–4 sentence summary based on whatever excerpts you have — these are representative samples of the document. Do NOT refuse general overview questions just because no single excerpt is titled "summary".
3. When you use information from an excerpt, cite its page using [page N] inline (e.g. "Revenue was $4.2M [page 3]"). Use exact page numbers from the excerpts. Summaries may cite multiple pages.
4. Answer in the SAME LANGUAGE the LATEST user message uses — even if earlier turns in this conversation were in a different language. Uzbek question → Uzbek-only answer. Russian → Russian-only. English → English-only. Never pad the response with a translation in another language. The document language may differ; translate excerpt content into the user's language inline.
5. Be concise — prefer 2–5 sentences unless the question explicitly asks for detail.

Document excerpts:
---
` + formatChunks(retrieved) + `
---`

	turns := make([]ChatTurn, 0, len(history)+1)
	for _, m := range history {
		if m.Role != "user" && m.Role != "assistant" {
			continue // skip system messages from history
		}
		turns = append(turns, ChatTurn{Role: m.Role, Content: m.Content})
	}
	turns = append(turns, ChatTurn{Role: "user", Content: question})

	return system, turns
}

// ChatTurn is a lightweight role+content pair. We don't import the llm
// package here because pdfchat is the lower layer in the dependency
// graph; the chat service maps these onto llm.Message at the call site.
type ChatTurn struct {
	Role    string
	Content string
}

// formatChunks renders the retrieved chunks for the system prompt. We
// keep each chunk tagged with its page number so the LLM can produce
// inline citations even when chunks from the same page appear together.
func formatChunks(chunks []models.PDFChunkWithScore) string {
	var sb strings.Builder
	for _, c := range chunks {
		fmt.Fprintf(&sb, "[page %d]\n%s\n\n", c.PageNumber, strings.TrimSpace(c.Content))
	}
	return strings.TrimSpace(sb.String())
}

// ─── Citation extraction ────────────────────────────────────────────────

var citationPattern = regexp.MustCompile(`\[page\s+(\d+)\]`)

// ExtractCitations pulls "[page N]" references out of an assistant
// response and links each one back to a retrieved chunk so the frontend
// can render a clickable badge that scrolls the PDF viewer to the right
// page.
//
// We match each cited page to the *highest-similarity* chunk on that page
// from the retrieval set — that's the chunk most likely to be what the
// LLM was actually referring to.
func ExtractCitations(answer string, retrieved []models.PDFChunkWithScore) []models.ChatCitation {
	if answer == "" {
		return nil
	}
	pages := map[int]string{} // page → best chunk_id from retrieval
	for _, c := range retrieved {
		if existing, ok := pages[c.PageNumber]; !ok || c.Score > scoreFor(retrieved, existing) {
			pages[c.PageNumber] = c.ID
		}
	}

	seen := map[int]bool{}
	out := make([]models.ChatCitation, 0)
	for _, m := range citationPattern.FindAllStringSubmatch(answer, -1) {
		page, err := strconv.Atoi(m[1])
		if err != nil || seen[page] {
			continue
		}
		seen[page] = true
		out = append(out, models.ChatCitation{
			Page:    page,
			ChunkID: pages[page], // may be "" if LLM cited a page not in retrieval
		})
	}
	return out
}

// scoreFor returns the similarity score for a given chunk_id in a
// retrieval set, or -1 if not found. Used by ExtractCitations to pick
// the best chunk on a multi-chunk page.
func scoreFor(chunks []models.PDFChunkWithScore, id string) float32 {
	for _, c := range chunks {
		if c.ID == id {
			return c.Score
		}
	}
	return -1
}
