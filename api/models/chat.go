package models

import "time"

// ChatSession is one PDF the user is chatting with. Created from
// POST /api/chat/sessions and indexed asynchronously by the worker.
type ChatSession struct {
	ID         string    `json:"id"`
	UserID     *string   `json:"user_id,omitempty"`
	FileID     string    `json:"file_id"`
	Title      string    `json:"title,omitempty"`
	Summary    string    `json:"summary,omitempty"`
	Status     string    `json:"status"` // indexing | ready | failed
	Error      string    `json:"error,omitempty"`
	PageCount  int       `json:"page_count,omitempty"`
	ChunkCount int       `json:"chunk_count,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// ChatCreateSessionRequest is the body for POST /api/chat/sessions.
type ChatCreateSessionRequest struct {
	FileID string `json:"file_id" binding:"required"`
}

// ChatMessage is one turn (user or assistant) in a session.
type ChatMessage struct {
	ID         string          `json:"id"`
	SessionID  string          `json:"session_id"`
	Role       string          `json:"role"` // user | assistant | system
	Content    string          `json:"content"`
	Citations  []ChatCitation  `json:"citations,omitempty"`
	TokensIn   int             `json:"tokens_in,omitempty"`
	TokensOut  int             `json:"tokens_out,omitempty"`
	LatencyMs  int             `json:"latency_ms,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
}

// ChatCitation links an answer span back to a source chunk + page so the
// frontend can render a clickable [page N] badge.
type ChatCitation struct {
	Page    int    `json:"page"`
	ChunkID string `json:"chunk_id,omitempty"`
}

// ChatSendMessageRequest is the body for POST /api/chat/sessions/<id>/messages.
type ChatSendMessageRequest struct {
	Content string `json:"content" binding:"required"`
}

// ChatSessionDetail is what GET /api/chat/sessions/<id> returns —
// includes suggested questions to power the sidebar prompts.
type ChatSessionDetail struct {
	ChatSession
	Suggestions []string `json:"suggested_questions,omitempty"`
}

// ChatSuggestion is the persistence row for chat_suggestions.
type ChatSuggestion struct {
	ID        string `json:"id"`
	SessionID string `json:"session_id"`
	Question  string `json:"question"`
	SortOrder int    `json:"sort_order"`
}

// PDFChunk is a stored embedded segment of the PDF. Embedding is omitted
// from JSON because the API never returns raw vectors to clients.
type PDFChunk struct {
	ID          string    `json:"id"`
	SessionID   string    `json:"session_id"`
	PageNumber  int       `json:"page_number"`
	ChunkIndex  int       `json:"chunk_index"`
	Content     string    `json:"content"`
	TokenCount  int       `json:"token_count,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// PDFChunkWithScore is what nearest-neighbour retrieval returns — a chunk
// plus the cosine similarity score (1.0 = identical, 0.0 = orthogonal).
type PDFChunkWithScore struct {
	PDFChunk
	Score float32 `json:"score"`
}
