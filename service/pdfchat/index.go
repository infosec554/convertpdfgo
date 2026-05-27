package pdfchat

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/gotenberg"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/service/llm"
	"convertpdfgo/storage"
)

// Indexer turns an uploaded PDF into a chat-ready session: extract → chunk
// → embed → save chunks → ask the LLM for a TL;DR + starter questions.
//
// The whole thing runs from a single goroutine inside the asynq worker
// process; no callbacks, no progress events — the session status (`indexing`
// → `ready` / `failed`) is the source of truth the frontend polls.
type Indexer struct {
	stg     storage.IStorage
	llm     llm.Provider
	log     logger.ILogger
	got     gotenberg.Client // nil-safe: only used for non-PDF uploads
}

func NewIndexer(stg storage.IStorage, p llm.Provider, log logger.ILogger, got gotenberg.Client) *Indexer {
	return &Indexer{stg: stg, llm: p, log: log, got: got}
}

// Run executes the full indexing pipeline for the given session.
// On any failure we mark the session as `failed` with a short error string
// the UI can render, and return the error so the asynq task records the
// failure too.
//
// `inputPath` is the absolute path to the uploaded file on disk. If the
// file isn't a PDF, we convert it (Word/Excel/PPT/HTML → PDF via
// gotenberg) before the rest of the pipeline runs.
func (ix *Indexer) Run(ctx context.Context, sessionID, inputPath string) error {
	pdfPath, err := ix.ensurePDF(ctx, sessionID, inputPath)
	if err != nil {
		return ix.markFailed(ctx, sessionID, fmt.Errorf("convert: %w", err))
	}

	// 1) Extract text page-by-page.
	pages, err := ExtractPages(ctx, pdfPath)
	if err != nil {
		return ix.markFailed(ctx, sessionID, fmt.Errorf("extract: %w", err))
	}
	if len(pages) == 0 {
		return ix.markFailed(ctx, sessionID, errors.New("pdf has no extractable text — try OCR"))
	}

	// 2) Chunk per page.
	chunks := ChunkPages(pages)
	if len(chunks) == 0 {
		return ix.markFailed(ctx, sessionID, errors.New("chunking produced no output"))
	}

	// 3) Embed every chunk in parallel.
	texts := make([]string, len(chunks))
	for i, c := range chunks {
		texts[i] = c.Content
	}
	vectors, err := ix.llm.Embed(ctx, texts)
	if err != nil {
		return ix.markFailed(ctx, sessionID, fmt.Errorf("embed: %w", err))
	}
	if len(vectors) != len(chunks) {
		return ix.markFailed(ctx, sessionID, errors.New("embedding count mismatch"))
	}

	// 4) Persist chunks + embeddings (single multi-row INSERT).
	pdfChunks := make([]models.PDFChunk, len(chunks))
	for i, c := range chunks {
		pdfChunks[i] = models.PDFChunk{
			ID:         uuid.NewString(),
			SessionID:  sessionID,
			PageNumber: c.Page,
			ChunkIndex: c.Index,
			Content:    c.Content,
			TokenCount: roughTokenCount(c.Content),
		}
	}
	if err := ix.stg.Chat().SaveChunks(ctx, sessionID, pdfChunks, vectors); err != nil {
		return ix.markFailed(ctx, sessionID, fmt.Errorf("save chunks: %w", err))
	}

	// 5) Generate a TL;DR summary + 4-6 starter questions in one LLM call.
	// We feed the first ~6 pages so the model has enough context to
	// produce something useful without blowing the token budget.
	summary, suggestions, title, err := ix.generateMetadata(ctx, pages)
	if err != nil {
		// Metadata is nice-to-have; if it fails we still mark the
		// session ready (the user can chat regardless) but log it.
		ix.log.Warning("pdfchat: metadata generation failed", logger.Error(err))
	} else if len(suggestions) > 0 {
		if err := ix.stg.Chat().SaveSuggestions(ctx, sessionID, suggestions); err != nil {
			ix.log.Warning("pdfchat: save suggestions", logger.Error(err))
		}
	}

	// 6) Finalise the session row.
	return ix.stg.Chat().UpdateSessionIndexing(
		ctx, sessionID, "ready", "", title, summary,
		len(pages), len(pdfChunks),
	)
}

// generateMetadata asks the LLM to produce both the summary and a list of
// suggested follow-up questions in a single round-trip. Combining them
// saves one LLM call per PDF, which matters under the free-tier rate
// limit.
func (ix *Indexer) generateMetadata(ctx context.Context, pages []Page) (summary string, suggestions []string, title string, err error) {
	// Concatenate the first ~6 pages so the model has enough context.
	const maxPagesForSummary = 6
	var preview strings.Builder
	for i, p := range pages {
		if i >= maxPagesForSummary {
			break
		}
		fmt.Fprintf(&preview, "[page %d]\n%s\n\n", p.Number, p.Text)
	}

	prompt := `You are summarising a PDF document for a chat assistant. Read the document excerpt below and produce a response in this exact format:

TITLE: <one short title, max 10 words>

SUMMARY: <2-3 sentences summarising what the document is about>

QUESTIONS:
1. <starter question a reader might ask>
2. <starter question>
3. <starter question>
4. <starter question>

Document excerpt:
---
` + preview.String() + `---

Do not add any extra commentary. Follow the format exactly.`

	ch, err := ix.llm.Chat(ctx, []llm.Message{{Role: llm.RoleUser, Content: prompt}}, llm.ChatOpts{
		Temperature: 0.3,
		MaxTokens:   500,
	})
	if err != nil {
		return "", nil, "", err
	}
	var sb strings.Builder
	for c := range ch {
		if c.Err != nil {
			return "", nil, "", c.Err
		}
		sb.WriteString(c.Delta)
	}

	title, summary, suggestions = parseMetadata(sb.String())
	return summary, suggestions, title, nil
}

// parseMetadata pulls the TITLE / SUMMARY / QUESTIONS sections out of the
// LLM's structured response. We're forgiving with whitespace and bullet
// styles since LLMs occasionally veer (e.g. "- " instead of "1. ").
func parseMetadata(raw string) (title, summary string, questions []string) {
	lines := strings.Split(raw, "\n")
	section := ""
	for _, ln := range lines {
		trim := strings.TrimSpace(ln)
		switch {
		case strings.HasPrefix(strings.ToUpper(trim), "TITLE:"):
			title = strings.TrimSpace(trim[len("TITLE:"):])
			section = "title"
		case strings.HasPrefix(strings.ToUpper(trim), "SUMMARY:"):
			summary = strings.TrimSpace(trim[len("SUMMARY:"):])
			section = "summary"
		case strings.HasPrefix(strings.ToUpper(trim), "QUESTIONS:"):
			section = "questions"
		case trim == "":
			continue
		default:
			switch section {
			case "summary":
				summary += " " + trim
			case "questions":
				q := stripBullet(trim)
				if q != "" {
					questions = append(questions, q)
				}
			}
		}
	}
	summary = strings.TrimSpace(summary)
	if len(questions) > 6 {
		questions = questions[:6]
	}
	return
}

// stripBullet removes leading "1. ", "1) ", "- ", "• " etc. from an LLM-
// generated list item.
func stripBullet(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// numbered: "1. ", "12) "
	for i, r := range s {
		if r >= '0' && r <= '9' {
			continue
		}
		if i > 0 && (r == '.' || r == ')') {
			return strings.TrimSpace(s[i+1:])
		}
		break
	}
	// bullet: "- ", "• ", "* "
	if len(s) >= 2 && (s[0] == '-' || s[0] == '*') && s[1] == ' ' {
		return strings.TrimSpace(s[2:])
	}
	if strings.HasPrefix(s, "• ") {
		return strings.TrimSpace(s[len("• "):])
	}
	return s
}

func (ix *Indexer) markFailed(ctx context.Context, sessionID string, err error) error {
	msg := err.Error()
	if len(msg) > 500 {
		msg = msg[:500]
	}
	if updateErr := ix.stg.Chat().UpdateSessionIndexing(ctx, sessionID, "failed", msg, "", "", 0, 0); updateErr != nil {
		ix.log.Error("pdfchat: failed to record failure", logger.Error(updateErr))
	}
	return err
}

// roughTokenCount approximates token count for budget tracking. Real
// tokenisers are model-specific (Gemini's is closer to ~3-4 chars/token);
// this is good enough for the chunk-budget heuristics that don't need to
// be exact.
func roughTokenCount(s string) int {
	return (len(s) + 3) / 4
}

// ─── Multi-format conversion ────────────────────────────────────────────

// ensurePDF returns a path to a PDF version of inputPath. If the input is
// already a PDF, it's returned unchanged. For Word/Excel/PowerPoint/HTML
// uploads we shell out to the gotenberg client (which proxies LibreOffice
// or Chromium internally), persist the resulting PDF as a fresh files
// row, and retarget the chat session at it so the PDF viewer pulls the
// converted file. Anything else is rejected with a clear error.
//
// The converted PDF lives at `uploads/<uuid>.pdf` — same on-disk layout
// as a regular upload, so the existing 5-minute cleanup job sweeps it
// too.
func (ix *Indexer) ensurePDF(ctx context.Context, sessionID, inputPath string) (string, error) {
	ext := strings.ToLower(filepath.Ext(inputPath))
	if ext == ".pdf" {
		return inputPath, nil
	}
	if ix.got == nil {
		return "", fmt.Errorf("gotenberg client missing; cannot convert %s", ext)
	}

	// Pick the right gotenberg endpoint based on the upload's extension.
	// "html" gets Chromium; Office formats get LibreOffice. CSV is
	// handled by ExcelToPDF — LibreOffice opens it as a spreadsheet.
	var convert func(context.Context, string, string) (int64, error)
	switch ext {
	case ".doc", ".docx", ".rtf", ".odt":
		convert = ix.got.WordToPDF
	case ".xls", ".xlsx", ".csv", ".ods":
		convert = ix.got.ExcelToPDF
	case ".ppt", ".pptx", ".odp":
		convert = ix.got.PowerPointToPDF
	case ".html", ".htm":
		convert = ix.got.HTMLToPDF
	default:
		return "", fmt.Errorf("unsupported file type %q — upload a PDF or Word/Excel/PowerPoint/HTML", ext)
	}

	outID := uuid.NewString()
	outPath := filepath.Join("uploads", outID+".pdf")
	if err := os.MkdirAll("uploads", 0o755); err != nil {
		return "", fmt.Errorf("prepare uploads dir: %w", err)
	}
	if _, err := convert(ctx, inputPath, outPath); err != nil {
		return "", fmt.Errorf("gotenberg %s: %w", ext, err)
	}

	// Persist the converted file as a fresh row so /file/<id>/download
	// can serve it later (PDF viewer + Markdown export both depend on
	// the chat session pointing at a downloadable file).
	fi, err := os.Stat(outPath)
	if err != nil {
		return "", fmt.Errorf("stat converted: %w", err)
	}
	sess, err := ix.stg.Chat().GetSession(ctx, sessionID)
	if err != nil {
		return "", fmt.Errorf("get session: %w", err)
	}
	// Keep the original filename but swap the extension — the chat
	// header shows this to the user.
	displayName := strings.TrimSuffix(filepath.Base(inputPath), ext) + ".pdf"
	if _, err := ix.stg.File().Save(ctx, models.File{
		ID:         outID,
		UserID:     sess.UserID,
		FileName:   displayName,
		FilePath:   outPath,
		FileType:   "application/pdf",
		FileSize:   fi.Size(),
		UploadedAt: time.Now(),
	}); err != nil {
		return "", fmt.Errorf("save converted file row: %w", err)
	}
	if err := ix.stg.Chat().SetSessionFileID(ctx, sessionID, outID); err != nil {
		// Non-fatal: indexing can still proceed, the viewer will just
		// keep pointing at the original Office doc which fails to
		// render. Log loudly so we notice.
		ix.log.Warning("pdfchat: failed to retarget session.file_id", logger.Error(err))
	}
	return outPath, nil
}
