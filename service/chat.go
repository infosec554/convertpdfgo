package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/gotenberg"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/service/llm"
	"convertpdfgo/service/pdfchat"
	"convertpdfgo/storage"
)

// ChatService is the application-level entry point for the /chat tool.
// HTTP handlers call Create/Get/List/Delete + SendMessage; the asynq
// worker invokes Process to run the indexing pipeline.
//
// The service stays usable even when no LLM provider is configured — every
// method just returns ErrChatDisabled in that case. Keeps boot simple: the
// API doesn't refuse to start because GEMINI_API_KEY isn't set in dev.
type ChatService interface {
	CreateSession(ctx context.Context, fileID string, userID *string) (*models.ChatSession, error)
	GetSession(ctx context.Context, id string) (*models.ChatSessionDetail, error)
	ListSessions(ctx context.Context, userID *string) ([]models.ChatSession, error)
	DeleteSession(ctx context.Context, id string) error
	// Process runs the indexing pipeline for a session. Called by the
	// asynq worker via the TaskChatIndex handler.
	Process(ctx context.Context, sessionID string) error

	// ListMessages returns the chat history for a session, oldest first.
	ListMessages(ctx context.Context, sessionID string) ([]models.ChatMessage, error)

	// SendMessage streams an assistant reply token-by-token via the
	// returned channel. The first event is StreamMeta (with the user's
	// stored message ID + retrieved chunk citations); subsequent events
	// are StreamToken; the final event is StreamDone (with usage stats).
	// On any error the channel emits StreamError and closes.
	//
	// Optional `attachments` (e.g. an image the user dropped into the
	// chat input) are passed through to Gemini's multimodal endpoint
	// alongside the text content.
	SendMessage(ctx context.Context, sessionID, content string, attachments []llm.Attachment) (<-chan ChatStreamEvent, error)
}

// ChatStreamEvent is one frame of the SendMessage stream. Exactly one of
// the typed fields is populated per event; Type tells the consumer which.
type ChatStreamEvent struct {
	Type      string                  // "meta" | "token" | "citations" | "done" | "error"
	UserMsgID string                  // populated on Meta
	Retrieved []models.PDFChunkWithScore // populated on Meta
	Delta     string                  // populated on Token
	Citations []models.ChatCitation   // populated on Citations
	AssistantMsgID string             // populated on Done
	TokensIn  int                     // populated on Done
	TokensOut int                     // populated on Done
	LatencyMs int                     // populated on Done
	Err       error                   // populated on Error
}

// ErrChatDisabled is returned by every ChatService method when the LLM
// provider couldn't be initialised at boot (missing API key etc.).
var ErrChatDisabled = errors.New("chat: feature disabled (LLM provider not configured)")

type chatService struct {
	stg     storage.IStorage
	log     logger.ILogger
	llm     llm.Provider     // nil when disabled
	indexer *pdfchat.Indexer // nil when disabled
}

// NewChatService boots the chat feature. The gotenberg client is needed
// because the indexer converts Word/Excel/PowerPoint uploads to PDF
// before chunking — that's how the /chat-word, /chat-excel, /chat-ppt
// dedicated tool pages share the single chat pipeline.
func NewChatService(stg storage.IStorage, log logger.ILogger, gotClient gotenberg.Client) ChatService {
	p, err := llm.FromEnv()
	if err != nil {
		log.Warning("chat: LLM provider not configured — chat tool will return ErrChatDisabled",
			logger.Error(err))
		return &chatService{stg: stg, log: log}
	}
	return &chatService{
		stg:     stg,
		log:     log,
		llm:     p,
		indexer: pdfchat.NewIndexer(stg, p, log, gotClient),
	}
}

// LLM exposes the underlying provider so handlers that need to stream a
// reply (chat messages — D6/D7) can borrow it without re-loading env.
// Returns nil when the service is disabled.
func (c *chatService) LLM() llm.Provider { return c.llm }

func (c *chatService) enabled() bool { return c.llm != nil && c.indexer != nil }

// ─── CreateSession ──────────────────────────────────────────────────────

func (c *chatService) CreateSession(ctx context.Context, fileID string, userID *string) (*models.ChatSession, error) {
	if !c.enabled() {
		return nil, ErrChatDisabled
	}
	// Confirm the file exists and the caller can see it before creating
	// the session — otherwise the indexer would just fail later with a
	// useless "no rows" error.
	if _, err := c.stg.File().GetByID(ctx, fileID); err != nil {
		return nil, fmt.Errorf("chat: file not found: %w", err)
	}

	sess := &models.ChatSession{
		FileID: fileID,
		UserID: userID,
		Status: "indexing",
	}
	if err := c.stg.Chat().CreateSession(ctx, sess); err != nil {
		return nil, fmt.Errorf("chat: create session: %w", err)
	}
	return sess, nil
}

// ─── GetSession ─────────────────────────────────────────────────────────

func (c *chatService) GetSession(ctx context.Context, id string) (*models.ChatSessionDetail, error) {
	if !c.enabled() {
		return nil, ErrChatDisabled
	}
	sess, err := c.stg.Chat().GetSession(ctx, id)
	if err != nil {
		return nil, err
	}
	suggestions, err := c.stg.Chat().GetSuggestions(ctx, id)
	if err != nil {
		c.log.Warning("chat: load suggestions failed", logger.Error(err))
	}
	return &models.ChatSessionDetail{
		ChatSession: *sess,
		Suggestions: suggestions,
	}, nil
}

// ─── ListSessions / DeleteSession ───────────────────────────────────────

func (c *chatService) ListSessions(ctx context.Context, userID *string) ([]models.ChatSession, error) {
	if !c.enabled() {
		return nil, ErrChatDisabled
	}
	return c.stg.Chat().ListSessions(ctx, userID, 50)
}

func (c *chatService) DeleteSession(ctx context.Context, id string) error {
	if !c.enabled() {
		return ErrChatDisabled
	}
	return c.stg.Chat().DeleteSession(ctx, id)
}

// ─── Process (worker entry point) ───────────────────────────────────────

// Process is invoked by the asynq worker. It looks up the session, finds
// the underlying file path on disk, and hands off to the indexer.
func (c *chatService) Process(ctx context.Context, sessionID string) error {
	if !c.enabled() {
		return ErrChatDisabled
	}
	sess, err := c.stg.Chat().GetSession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("chat: get session: %w", err)
	}
	file, err := c.stg.File().GetByID(ctx, sess.FileID)
	if err != nil {
		return fmt.Errorf("chat: get file: %w", err)
	}
	return c.indexer.Run(ctx, sessionID, file.FilePath)
}

// ─── ListMessages ───────────────────────────────────────────────────────

func (c *chatService) ListMessages(ctx context.Context, sessionID string) ([]models.ChatMessage, error) {
	if !c.enabled() {
		return nil, ErrChatDisabled
	}
	return c.stg.Chat().ListMessages(ctx, sessionID, 200)
}

// ─── SendMessage (streaming) ────────────────────────────────────────────

// chatHistoryDepth is how many previous turns we feed back into the LLM
// alongside the new question. 6 ≈ 3 user+assistant pairs — keeps prompt
// size manageable while preserving short-range context like "what about
// that last number?".
const chatHistoryDepth = 6

// retrievalK is the number of pdf_chunks we pull from the HNSW index for
// each question. Tuned with the token budget in mind: 7 chunks × ~175
// tokens ≈ 1.2K context tokens, well under Gemini Flash's free-tier RPM.
const retrievalK = 7

// SendMessage orchestrates the full answer pipeline: persist the user
// turn, embed the question, retrieve relevant chunks, stream the LLM
// reply to the caller, extract citations from the assembled response,
// and persist the assistant turn.
//
// Errors before streaming starts come back as the second return value.
// Errors during streaming surface as a ChatStreamEvent{Type:"error"} on
// the channel just before it closes.
func (c *chatService) SendMessage(ctx context.Context, sessionID, content string, attachments []llm.Attachment) (<-chan ChatStreamEvent, error) {
	if !c.enabled() {
		return nil, ErrChatDisabled
	}

	content = strings.TrimSpace(content)
	// Allow attachment-only turns (drop an image, no caption) — only
	// reject when *everything* is empty.
	if content == "" && len(attachments) == 0 {
		return nil, errors.New("chat: empty message")
	}

	sess, err := c.stg.Chat().GetSession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("chat: get session: %w", err)
	}
	if sess.Status != "ready" {
		return nil, fmt.Errorf("chat: session not ready (status=%s)", sess.Status)
	}

	// Persist the user turn up-front so it shows in history even if the
	// LLM call fails downstream.
	userMsg := &models.ChatMessage{
		ID:        uuid.NewString(),
		SessionID: sessionID,
		Role:      "user",
		Content:   content,
		CreatedAt: time.Now(),
	}
	if err := c.stg.Chat().AppendMessage(ctx, userMsg); err != nil {
		return nil, fmt.Errorf("chat: save user message: %w", err)
	}

	// Embed the question. Single-text Embed call — fast.
	embeds, err := c.llm.Embed(ctx, []string{content})
	if err != nil {
		return nil, fmt.Errorf("chat: embed question: %w", err)
	}
	if len(embeds) != 1 {
		return nil, errors.New("chat: embedding API returned wrong count")
	}

	// Retrieve top-K relevant chunks.
	retrieved, err := c.stg.Chat().SearchChunks(ctx, sessionID, embeds[0], retrievalK)
	if err != nil {
		return nil, fmt.Errorf("chat: retrieve chunks: %w", err)
	}

	// Pull recent history for short-term context. Drop the user message
	// we just inserted — we'll add it back as the final turn in the
	// prompt builder.
	history, err := c.stg.Chat().ListMessages(ctx, sessionID, chatHistoryDepth+1)
	if err != nil {
		c.log.Warning("chat: load history failed", logger.Error(err))
		history = nil
	}
	if n := len(history); n > 0 && history[n-1].ID == userMsg.ID {
		history = history[:n-1]
	}
	if len(history) > chatHistoryDepth {
		history = history[len(history)-chatHistoryDepth:]
	}

	system, turns := pdfchat.BuildPrompt(retrieved, history, content)

	// Convert pdfchat turns → llm.Message. Attach any user-supplied
	// images to the final turn (the one carrying the new question) so
	// Gemini gets text + image in a single multimodal user message.
	msgs := make([]llm.Message, len(turns))
	for i, t := range turns {
		role := llm.RoleUser
		if t.Role == "assistant" {
			role = llm.RoleAssistant
		}
		msg := llm.Message{Role: role, Content: t.Content}
		if i == len(turns)-1 && len(attachments) > 0 {
			msg.Attachments = attachments
		}
		msgs[i] = msg
	}

	// Kick off the LLM stream.
	started := time.Now()
	stream, err := c.llm.Chat(ctx, msgs, llm.ChatOpts{
		System:      system,
		Temperature: 0.2,
		MaxTokens:   1024,
	})
	if err != nil {
		return nil, fmt.Errorf("chat: llm stream: %w", err)
	}

	out := make(chan ChatStreamEvent, 16)

	go func() {
		defer close(out)

		// First event — meta with retrieved chunks (so frontend can pre-
		// render the citation candidates) and the user message ID.
		out <- ChatStreamEvent{
			Type:      "meta",
			UserMsgID: userMsg.ID,
			Retrieved: retrieved,
		}

		var (
			fullText  strings.Builder
			tokensIn  int
			tokensOut int
			finishErr error
		)
		for chunk := range stream {
			if chunk.Err != nil {
				finishErr = chunk.Err
				break
			}
			if chunk.Delta != "" {
				fullText.WriteString(chunk.Delta)
				out <- ChatStreamEvent{Type: "token", Delta: chunk.Delta}
			}
			// FinishReason arrives only on the final chunk.
			if chunk.FinishReason != "" {
				tokensIn = chunk.TokensIn
				tokensOut = chunk.TokensOut
			}
		}

		if finishErr != nil {
			out <- ChatStreamEvent{Type: "error", Err: finishErr}
			return
		}

		// Extract citations from the full response.
		answer := fullText.String()
		citations := pdfchat.ExtractCitations(answer, retrieved)
		if len(citations) > 0 {
			out <- ChatStreamEvent{Type: "citations", Citations: citations}
		}

		// Persist the assistant turn. Use a fresh context — the request
		// context may already be canceled if the client disconnected
		// mid-stream, and we still want history on the next page load.
		saveCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		assistant := &models.ChatMessage{
			ID:        uuid.NewString(),
			SessionID: sessionID,
			Role:      "assistant",
			Content:   answer,
			Citations: citations,
			TokensIn:  tokensIn,
			TokensOut: tokensOut,
			LatencyMs: int(time.Since(started).Milliseconds()),
			CreatedAt: time.Now(),
		}
		if err := c.stg.Chat().AppendMessage(saveCtx, assistant); err != nil {
			c.log.Warning("chat: save assistant message", logger.Error(err))
		}

		out <- ChatStreamEvent{
			Type:           "done",
			AssistantMsgID: assistant.ID,
			TokensIn:       tokensIn,
			TokensOut:      tokensOut,
			LatencyMs:      assistant.LatencyMs,
		}
	}()

	return out, nil
}
