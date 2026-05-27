package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
)

// chatRepo backs the IChatStorage interface — sessions, messages,
// suggestions, and pdf_chunks all live here because they share the same
// pgvector-enabled DB connection and we don't want to fan out repo types
// for what amounts to a single feature.
type chatRepo struct {
	db  *pgxpool.Pool
	log logger.ILogger
}

func NewChatRepo(db *pgxpool.Pool, log logger.ILogger) *chatRepo {
	return &chatRepo{db: db, log: log}
}

// ─── Sessions ───────────────────────────────────────────────────────────

func (r *chatRepo) CreateSession(ctx context.Context, s *models.ChatSession) error {
	if s.ID == "" {
		s.ID = uuid.NewString()
	}
	if s.Status == "" {
		s.Status = "indexing"
	}
	now := time.Now()
	s.CreatedAt = now
	s.UpdatedAt = now

	var userID any
	if s.UserID != nil && *s.UserID != "" {
		userID = *s.UserID
	}

	_, err := r.db.Exec(ctx, `
		INSERT INTO chat_sessions (id, user_id, file_id, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $5)
	`, s.ID, userID, s.FileID, s.Status, now)
	if err != nil {
		r.log.Error("chatRepo.CreateSession", logger.Error(err))
	}
	return err
}

func (r *chatRepo) GetSession(ctx context.Context, id string) (*models.ChatSession, error) {
	var s models.ChatSession
	var title, summary, errStr *string
	var pageCount, chunkCount *int

	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, file_id, title, summary, status, error,
		       page_count, chunk_count, created_at, updated_at
		FROM chat_sessions WHERE id = $1
	`, id).Scan(
		&s.ID, &s.UserID, &s.FileID, &title, &summary, &s.Status, &errStr,
		&pageCount, &chunkCount, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("chat session not found")
		}
		return nil, err
	}
	if title != nil {
		s.Title = *title
	}
	if summary != nil {
		s.Summary = *summary
	}
	if errStr != nil {
		s.Error = *errStr
	}
	if pageCount != nil {
		s.PageCount = *pageCount
	}
	if chunkCount != nil {
		s.ChunkCount = *chunkCount
	}
	return &s, nil
}

// UpdateSessionIndexing finalises a session after the indexing task has
// completed (or failed). The metadata fields are only meaningful on success.
func (r *chatRepo) UpdateSessionIndexing(ctx context.Context, id, status, errMsg, title, summary string, pageCount, chunkCount int) error {
	_, err := r.db.Exec(ctx, `
		UPDATE chat_sessions
		SET status = $1, error = NULLIF($2, ''), title = NULLIF($3, ''),
		    summary = NULLIF($4, ''), page_count = $5, chunk_count = $6,
		    updated_at = NOW()
		WHERE id = $7
	`, status, errMsg, title, summary, pageCount, chunkCount, id)
	return err
}

func (r *chatRepo) ListSessions(ctx context.Context, userID *string, limit int) ([]models.ChatSession, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	var rows pgx.Rows
	var err error
	if userID == nil {
		// Guests can't list — return empty rather than leaking other
		// guests' sessions.
		return []models.ChatSession{}, nil
	}
	rows, err = r.db.Query(ctx, `
		SELECT id, user_id, file_id, COALESCE(title,''), COALESCE(summary,''),
		       status, COALESCE(error,''), COALESCE(page_count, 0),
		       COALESCE(chunk_count, 0), created_at, updated_at
		FROM chat_sessions WHERE user_id = $1
		ORDER BY updated_at DESC LIMIT $2
	`, *userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]models.ChatSession, 0, limit)
	for rows.Next() {
		var s models.ChatSession
		if err := rows.Scan(&s.ID, &s.UserID, &s.FileID, &s.Title, &s.Summary,
			&s.Status, &s.Error, &s.PageCount, &s.ChunkCount,
			&s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *chatRepo) DeleteSession(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM chat_sessions WHERE id = $1`, id)
	return err
}

// SetSessionFileID retargets the session to a different file row. The
// indexer calls this after converting a Word/Excel/PPT upload to PDF so
// the PDF viewer (and any future re-indexing) sees the converted file
// directly. Updated_at moves too so the sidebar reflects the change.
func (r *chatRepo) SetSessionFileID(ctx context.Context, sessionID, fileID string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE chat_sessions SET file_id = $1, updated_at = NOW()
		WHERE id = $2
	`, fileID, sessionID)
	return err
}

// ─── Messages ───────────────────────────────────────────────────────────

func (r *chatRepo) AppendMessage(ctx context.Context, m *models.ChatMessage) error {
	if m.ID == "" {
		m.ID = uuid.NewString()
	}
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now()
	}
	var citationsJSON []byte
	if len(m.Citations) > 0 {
		b, err := json.Marshal(m.Citations)
		if err != nil {
			return fmt.Errorf("marshal citations: %w", err)
		}
		citationsJSON = b
	}
	_, err := r.db.Exec(ctx, `
		INSERT INTO chat_messages
			(id, session_id, role, content, citations, tokens_in, tokens_out, latency_ms, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, m.ID, m.SessionID, m.Role, m.Content, citationsJSON,
		m.TokensIn, m.TokensOut, m.LatencyMs, m.CreatedAt)
	if err != nil {
		r.log.Error("chatRepo.AppendMessage", logger.Error(err))
	}
	return err
}

func (r *chatRepo) ListMessages(ctx context.Context, sessionID string, limit int) ([]models.ChatMessage, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := r.db.Query(ctx, `
		SELECT id, session_id, role, content, citations,
		       COALESCE(tokens_in,0), COALESCE(tokens_out,0), COALESCE(latency_ms,0), created_at
		FROM chat_messages WHERE session_id = $1
		ORDER BY created_at ASC LIMIT $2
	`, sessionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]models.ChatMessage, 0, limit)
	for rows.Next() {
		var m models.ChatMessage
		var citationsJSON []byte
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content,
			&citationsJSON, &m.TokensIn, &m.TokensOut, &m.LatencyMs, &m.CreatedAt); err != nil {
			return nil, err
		}
		if len(citationsJSON) > 0 {
			_ = json.Unmarshal(citationsJSON, &m.Citations)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ─── Suggestions ────────────────────────────────────────────────────────

func (r *chatRepo) SaveSuggestions(ctx context.Context, sessionID string, qs []string) error {
	if len(qs) == 0 {
		return nil
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM chat_suggestions WHERE session_id = $1`, sessionID); err != nil {
		return err
	}
	for i, q := range qs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO chat_suggestions (id, session_id, question, sort_order)
			VALUES ($1, $2, $3, $4)
		`, uuid.NewString(), sessionID, q, i); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *chatRepo) GetSuggestions(ctx context.Context, sessionID string) ([]string, error) {
	rows, err := r.db.Query(ctx, `
		SELECT question FROM chat_suggestions
		WHERE session_id = $1 ORDER BY sort_order
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var q string
		if err := rows.Scan(&q); err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

// ─── PDF chunks (pgvector) ──────────────────────────────────────────────

// SaveChunks inserts a batch of chunks + embeddings. We do this in a
// single multi-VALUES statement so the indexer doesn't pay per-row round
// trip latency on a 1500-chunk PDF. pgx batches would also work but the
// VALUES list is simpler to reason about.
func (r *chatRepo) SaveChunks(ctx context.Context, sessionID string, chunks []models.PDFChunk, embeddings [][]float32) error {
	if len(chunks) != len(embeddings) {
		return errors.New("chunks/embeddings length mismatch")
	}
	if len(chunks) == 0 {
		return nil
	}

	// Build $1, $2, ..., placeholders for each row. 6 columns per row.
	const cols = 6
	args := make([]any, 0, len(chunks)*cols)
	values := make([]string, 0, len(chunks))
	for i, c := range chunks {
		off := i*cols + 1
		values = append(values, fmt.Sprintf("($%d,$%d,$%d,$%d,$%d,$%d::vector)",
			off, off+1, off+2, off+3, off+4, off+5))
		if c.ID == "" {
			c.ID = uuid.NewString()
		}
		args = append(args,
			c.ID, sessionID, c.PageNumber, c.ChunkIndex, c.Content,
			vectorToLiteral(embeddings[i]))
	}

	query := `INSERT INTO pdf_chunks (id, session_id, page_number, chunk_index, content, embedding) VALUES ` +
		strings.Join(values, ",")

	if _, err := r.db.Exec(ctx, query, args...); err != nil {
		r.log.Error("chatRepo.SaveChunks", logger.Error(err))
		return err
	}
	return nil
}

// SearchChunks runs a cosine-distance kNN query against the HNSW index.
// Returns up to `limit` chunks ordered by similarity (most similar first).
// We compute similarity as 1 - cosine_distance so 1.0 = perfect match.
func (r *chatRepo) SearchChunks(ctx context.Context, sessionID string, queryEmbedding []float32, limit int) ([]models.PDFChunkWithScore, error) {
	if limit <= 0 || limit > 50 {
		limit = 7
	}
	rows, err := r.db.Query(ctx, `
		SELECT id, session_id, page_number, chunk_index, content,
		       COALESCE(token_count, 0), created_at,
		       1 - (embedding <=> $1::vector) AS similarity
		FROM pdf_chunks
		WHERE session_id = $2
		ORDER BY embedding <=> $1::vector
		LIMIT $3
	`, vectorToLiteral(queryEmbedding), sessionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]models.PDFChunkWithScore, 0, limit)
	for rows.Next() {
		var c models.PDFChunkWithScore
		if err := rows.Scan(&c.ID, &c.SessionID, &c.PageNumber, &c.ChunkIndex,
			&c.Content, &c.TokenCount, &c.CreatedAt, &c.Score); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// vectorToLiteral formats a slice as pgvector's "[1.23,4.56,...]" literal.
// fmt.Sprintf with %v gives "[1.23 4.56]" with spaces which pgvector rejects
// at parse time, so we go manual.
func vectorToLiteral(v []float32) string {
	if len(v) == 0 {
		return "[]"
	}
	var sb strings.Builder
	sb.Grow(len(v) * 12)
	sb.WriteByte('[')
	for i, x := range v {
		if i > 0 {
			sb.WriteByte(',')
		}
		fmt.Fprintf(&sb, "%g", x)
	}
	sb.WriteByte(']')
	return sb.String()
}
