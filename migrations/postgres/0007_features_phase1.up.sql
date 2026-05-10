-- Phase 1 PDF features: watermark (extended), extract-text, n-up, url-to-pdf, metadata.
-- Watermark already has a barebones table from 0001; we extend it with the
-- richer params (image source, rotation, opacity, page range, mode flag).

ALTER TABLE watermark_jobs
    ADD COLUMN IF NOT EXISTS mode          VARCHAR(10)  NOT NULL DEFAULT 'text',
    ADD COLUMN IF NOT EXISTS image_file_id UUID         REFERENCES files(id),
    ADD COLUMN IF NOT EXISTS rotation      INTEGER      NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS opacity       NUMERIC(3,2) NOT NULL DEFAULT 0.40,
    ADD COLUMN IF NOT EXISTS pages         VARCHAR(64)  NOT NULL DEFAULT 'all';

-- Existing rows allowed empty watermark_text; ensure new code path can rely on it.
ALTER TABLE watermark_jobs
    ALTER COLUMN watermark_text DROP NOT NULL;

-- Extract text → TXT or per-page JSON.
CREATE TABLE IF NOT EXISTS extract_text_jobs (
    id             UUID PRIMARY KEY,
    user_id        UUID REFERENCES users(id),
    input_file_id  UUID NOT NULL REFERENCES files(id),
    format         VARCHAR(8)  NOT NULL DEFAULT 'txt',  -- 'txt' | 'json'
    output_file_id UUID REFERENCES files(id),
    status         VARCHAR(20) NOT NULL,
    created_at     TIMESTAMP DEFAULT NOW()
);

-- N-up imposition (multiple pages on a single sheet).
CREATE TABLE IF NOT EXISTS nup_jobs (
    id             UUID PRIMARY KEY,
    user_id        UUID REFERENCES users(id),
    input_file_id  UUID NOT NULL REFERENCES files(id),
    n              INTEGER NOT NULL,                    -- 2,4,6,9
    orientation    VARCHAR(10) NOT NULL DEFAULT 'portrait',
    border         BOOLEAN NOT NULL DEFAULT FALSE,
    output_file_id UUID REFERENCES files(id),
    status         VARCHAR(20) NOT NULL,
    created_at     TIMESTAMP DEFAULT NOW()
);

-- URL → PDF (Gotenberg /forms/chromium/convert/url).
CREATE TABLE IF NOT EXISTS url_to_pdf_jobs (
    id             UUID PRIMARY KEY,
    user_id        UUID REFERENCES users(id),
    url            TEXT        NOT NULL,
    page_size      VARCHAR(8)  NOT NULL DEFAULT 'A4',
    landscape      BOOLEAN     NOT NULL DEFAULT FALSE,
    wait_for       VARCHAR(64),
    output_file_id UUID REFERENCES files(id),
    status         VARCHAR(20) NOT NULL,
    created_at     TIMESTAMP DEFAULT NOW()
);

-- Metadata edit (in-place rewrite of /Title /Author /Subject /Keywords /Creator).
CREATE TABLE IF NOT EXISTS metadata_jobs (
    id             UUID PRIMARY KEY,
    user_id        UUID REFERENCES users(id),
    input_file_id  UUID NOT NULL REFERENCES files(id),
    title          TEXT,
    author         TEXT,
    subject        TEXT,
    keywords       TEXT,
    creator        TEXT,
    output_file_id UUID REFERENCES files(id),
    status         VARCHAR(20) NOT NULL,
    created_at     TIMESTAMP DEFAULT NOW()
);
