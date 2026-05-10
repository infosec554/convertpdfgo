-- Three job tables had repos and handlers but were never in any migration —
-- /api/pdf/header-footer, /api/pdf/qr-code and /api/pdf/detect-blank failed
-- with "relation does not exist" the moment anyone called them. Create them
-- now with FKs that already cascade (matches the 0008 contract for new
-- tables).

CREATE TABLE IF NOT EXISTS add_header_footer_jobs (
    id             UUID PRIMARY KEY,
    user_id        UUID         REFERENCES users(id),
    input_file_id  UUID NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    header_text    TEXT,
    footer_text    TEXT,
    font_size      INTEGER NOT NULL DEFAULT 12,
    font_color     VARCHAR(20) NOT NULL DEFAULT 'black',
    position       VARCHAR(20) NOT NULL DEFAULT 'center',
    output_file_id UUID         REFERENCES files(id) ON DELETE CASCADE,
    status         VARCHAR(20) NOT NULL,
    created_at     TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS qr_code_jobs (
    id             UUID PRIMARY KEY,
    user_id        UUID         REFERENCES users(id),
    input_file_id  UUID NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    qr_content     TEXT NOT NULL,
    position       VARCHAR(20) NOT NULL,
    size           INTEGER NOT NULL,
    output_file_id UUID         REFERENCES files(id) ON DELETE CASCADE,
    status         VARCHAR(20) NOT NULL,
    created_at     TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS detect_blank_pages_jobs (
    id            UUID PRIMARY KEY,
    user_id       UUID         REFERENCES users(id),
    input_file_id UUID NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    blank_pages   INTEGER[] DEFAULT ARRAY[]::INTEGER[],
    status        VARCHAR(20) NOT NULL,
    created_at    TIMESTAMP DEFAULT NOW()
);
