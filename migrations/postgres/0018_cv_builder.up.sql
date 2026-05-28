-- 0018_cv_builder.up.sql
-- AI Resume / CV Builder feature.
-- pgvector is already enabled by migration 0017 (PDF Chat AI).

-- Asosiy CV session. Har user bir nechta CV saqlay oladi.
CREATE TABLE cv_sessions (
    id              UUID PRIMARY KEY,
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title           TEXT NOT NULL DEFAULT 'Untitled CV',
    mode            VARCHAR(20) NOT NULL
                    CHECK (mode IN ('scratch', 'import', 'voice')),
    source_file_id  UUID REFERENCES files(id) ON DELETE SET NULL,
    template_id     VARCHAR(50) NOT NULL DEFAULT 'modern',
    locale          VARCHAR(10) NOT NULL DEFAULT 'en',
    status          VARCHAR(20) NOT NULL DEFAULT 'draft'
                    CHECK (status IN ('draft', 'parsing', 'ready', 'failed')),
    parse_error     TEXT,
    last_pdf_id     UUID REFERENCES files(id) ON DELETE SET NULL,
    created_at      TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMP NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_cv_sessions_user ON cv_sessions(user_id, updated_at DESC);

-- Section'lar: har CV uchun ko'p bo'lim. content JSONB chunki har bo'lim
-- turi farqli schema (personal vs experience vs skills va h.k.).
CREATE TABLE cv_sections (
    id              UUID PRIMARY KEY,
    cv_id           UUID NOT NULL REFERENCES cv_sessions(id) ON DELETE CASCADE,
    section_type    VARCHAR(30) NOT NULL,
    sort_order      INT NOT NULL DEFAULT 0,
    content         JSONB NOT NULL,
    embedding       vector(768),
    created_at      TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMP NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_cv_sections_cv ON cv_sections(cv_id, sort_order);
CREATE INDEX idx_cv_sections_embedding ON cv_sections
    USING hnsw (embedding vector_cosine_ops);

-- Template registry. Asosan code-side enum, lekin admin custom qo'shsa.
CREATE TABLE cv_templates (
    id              VARCHAR(50) PRIMARY KEY,
    name            TEXT NOT NULL,
    description     TEXT,
    is_ats_friendly BOOLEAN NOT NULL DEFAULT TRUE,
    preview_url     TEXT,
    html_path       TEXT NOT NULL,
    category        VARCHAR(20),
    sort_order      INT NOT NULL DEFAULT 0,
    is_public       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Seed the 10 MVP templates so the UI can list them on day one.
INSERT INTO cv_templates (id, name, category, is_ats_friendly, sort_order, html_path, description) VALUES
    ('modern',    'Modern',     'modern',    TRUE,  10, 'modern.html.tmpl',    'Orange accent, clean sans-serif (default).'),
    ('classic',   'Classic',    'classic',   TRUE,  20, 'classic.html.tmpl',   'Times New Roman, double-column header.'),
    ('minimal',   'Minimal',    'classic',   TRUE,  30, 'minimal.html.tmpl',   'Black & white, narrow margins.'),
    ('tech',      'Tech',       'modern',    TRUE,  40, 'tech.html.tmpl',      'Monospace headers, GitHub-style.'),
    ('creative',  'Creative',   'creative',  FALSE, 50, 'creative.html.tmpl',  'Color blocks, visual hierarchy.'),
    ('executive', 'Executive',  'classic',   TRUE,  60, 'executive.html.tmpl', 'Serif, large name, formal.'),
    ('academic',  'Academic',   'classic',   TRUE,  70, 'academic.html.tmpl',  'Publications + research focus.'),
    ('designer',  'Designer',   'creative',  FALSE, 80, 'designer.html.tmpl',  'Bold typography, asymmetric.'),
    ('two-col',   'Two Column', 'modern',    TRUE,  90, 'two-col.html.tmpl',   'Sidebar + main column.'),
    ('one-col',   'One Column', 'modern',    TRUE, 100, 'one-col.html.tmpl',   'Maximum text density.');

-- ATS score runs (each job description -> 0-100 score)
CREATE TABLE cv_ats_runs (
    id              UUID PRIMARY KEY,
    cv_id           UUID NOT NULL REFERENCES cv_sessions(id) ON DELETE CASCADE,
    job_title       TEXT,
    job_description TEXT NOT NULL,
    score           INT NOT NULL CHECK (score BETWEEN 0 AND 100),
    feedback        JSONB NOT NULL,
    created_at      TIMESTAMP NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_cv_ats_runs_cv ON cv_ats_runs(cv_id, created_at DESC);

-- AI Improve history — each call records 3 variants, which one user picked.
CREATE TABLE cv_ai_history (
    id              UUID PRIMARY KEY,
    cv_id           UUID NOT NULL REFERENCES cv_sessions(id) ON DELETE CASCADE,
    section_id      UUID REFERENCES cv_sections(id) ON DELETE CASCADE,
    field_path      TEXT NOT NULL,
    prompt          TEXT NOT NULL,
    suggestions     JSONB NOT NULL,
    tokens_in       INT,
    tokens_out      INT,
    created_at      TIMESTAMP NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_cv_ai_history_cv ON cv_ai_history(cv_id, created_at DESC);

-- Voice transcripts: text only (no audio bytes stored on disk).
CREATE TABLE cv_voice_segments (
    id              UUID PRIMARY KEY,
    cv_id           UUID NOT NULL REFERENCES cv_sessions(id) ON DELETE CASCADE,
    transcript      TEXT NOT NULL,
    parsed_section  VARCHAR(30),
    parsed_content  JSONB,
    locale          VARCHAR(10) NOT NULL DEFAULT 'en',
    created_at      TIMESTAMP NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_cv_voice_cv ON cv_voice_segments(cv_id, created_at);

-- Per-user AI call rate limits (CV Builder uses Improve heavily).
CREATE TABLE cv_rate_limits (
    user_id         UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    ai_calls_today  INT NOT NULL DEFAULT 0,
    ats_runs_today  INT NOT NULL DEFAULT 0,
    window_start    TIMESTAMP NOT NULL DEFAULT NOW()
);
