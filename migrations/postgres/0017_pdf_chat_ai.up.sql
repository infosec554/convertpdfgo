-- 0017_pdf_chat_ai.up.sql
-- PDF Chat AI feature — RAG-based chat with PDFs using Gemini.
-- Requires the postgres container to run pgvector/pgvector:pg15 (the stock
-- postgres:15 image does not ship the extension). docker-compose.yml is
-- updated alongside this migration.

CREATE EXTENSION IF NOT EXISTS vector;

-- One session per PDF the user wants to chat with.
CREATE TABLE chat_sessions (
    id          UUID PRIMARY KEY,
    user_id     UUID REFERENCES users(id) ON DELETE CASCADE,  -- NULL → guest
    file_id     UUID NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    title       TEXT,                                          -- auto-extracted from PDF metadata or first heading
    summary     TEXT,                                          -- auto TL;DR generated after indexing
    status      VARCHAR(20) NOT NULL DEFAULT 'indexing'
                CHECK (status IN ('indexing', 'ready', 'failed')),
    error       TEXT,                                          -- populated when status='failed'
    page_count  INT,
    chunk_count INT,
    created_at  TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_chat_sessions_user ON chat_sessions(user_id, created_at DESC);
CREATE INDEX idx_chat_sessions_file ON chat_sessions(file_id);

-- One row per user/assistant turn.
CREATE TABLE chat_messages (
    id          UUID PRIMARY KEY,
    session_id  UUID NOT NULL REFERENCES chat_sessions(id) ON DELETE CASCADE,
    role        VARCHAR(10) NOT NULL CHECK (role IN ('user', 'assistant', 'system')),
    content     TEXT NOT NULL,
    citations   JSONB,                                          -- [{"page":4,"chunk_id":"..."}]
    tokens_in   INT,
    tokens_out  INT,
    latency_ms  INT,
    created_at  TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_chat_messages_session ON chat_messages(session_id, created_at);

-- Embedded chunks of the PDF. Each session owns its chunks; deleting the
-- session cascades these.
--
-- vector(768): Gemini text-embedding-004 outputs 768-dimensional vectors.
-- If we ever switch to a different embedding model (e.g. OpenAI's 1536-dim
-- text-embedding-3-small), this needs a new migration + re-index.
CREATE TABLE pdf_chunks (
    id          UUID PRIMARY KEY,
    session_id  UUID NOT NULL REFERENCES chat_sessions(id) ON DELETE CASCADE,
    page_number INT NOT NULL,
    chunk_index INT NOT NULL,
    content     TEXT NOT NULL,
    embedding   vector(768),
    token_count INT,
    created_at  TIMESTAMP NOT NULL DEFAULT NOW()
);

-- HNSW index for fast cosine-similarity retrieval. Build is slower than
-- IVFFlat but search latency is much better, and we read >> write here.
CREATE INDEX idx_pdf_chunks_embedding ON pdf_chunks
    USING hnsw (embedding vector_cosine_ops);

CREATE INDEX idx_pdf_chunks_session ON pdf_chunks(session_id, page_number);

-- LLM-generated starter questions surfaced in the chat sidebar.
CREATE TABLE chat_suggestions (
    id          UUID PRIMARY KEY,
    session_id  UUID NOT NULL REFERENCES chat_sessions(id) ON DELETE CASCADE,
    question    TEXT NOT NULL,
    sort_order  INT NOT NULL DEFAULT 0,
    created_at  TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_chat_suggestions_session ON chat_suggestions(session_id, sort_order);

-- Simple per-IP-or-user rate limit table. Window resets are handled in app
-- code by comparing `window_start` to NOW().
CREATE TABLE chat_rate_limits (
    ip_or_user   TEXT PRIMARY KEY,
    count        INT NOT NULL DEFAULT 0,
    window_start TIMESTAMP NOT NULL DEFAULT NOW()
);
