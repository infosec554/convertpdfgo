-- OCR family of jobs (PDF_ROADMAP.md "OCR + Analysis").
--
-- Five of them produce a file (searchable PDF, txt, JSON); one is an
-- analytic endpoint whose result lives entirely on the job row
-- (ocr_detect_lang_jobs). Output FKs all CASCADE so the 5-min privacy
-- cleaner takes the job rows down with the files.

CREATE TABLE IF NOT EXISTS ocr_jobs (
    id             UUID PRIMARY KEY,
    user_id        UUID REFERENCES users(id),
    input_file_id  UUID NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    output_file_id UUID REFERENCES files(id) ON DELETE CASCADE,
    lang           VARCHAR(64) NOT NULL DEFAULT 'eng', -- "eng+uzb" syntax allowed
    force_ocr      BOOLEAN     NOT NULL DEFAULT FALSE,
    status         VARCHAR(20) NOT NULL CHECK (status IN ('pending','processing','done','failed')),
    created_at     TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS scan_to_pdf_jobs (
    id             UUID PRIMARY KEY,
    user_id        UUID REFERENCES users(id),
    input_file_ids UUID[] NOT NULL,
    output_file_id UUID REFERENCES files(id) ON DELETE CASCADE,
    lang           VARCHAR(64) NOT NULL DEFAULT 'eng',
    status         VARCHAR(20) NOT NULL CHECK (status IN ('pending','processing','done','failed')),
    created_at     TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS ocr_extract_text_jobs (
    id             UUID PRIMARY KEY,
    user_id        UUID REFERENCES users(id),
    input_file_id  UUID NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    output_file_id UUID REFERENCES files(id) ON DELETE CASCADE,
    lang           VARCHAR(64) NOT NULL DEFAULT 'eng',
    status         VARCHAR(20) NOT NULL CHECK (status IN ('pending','processing','done','failed')),
    created_at     TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS image_to_text_jobs (
    id             UUID PRIMARY KEY,
    user_id        UUID REFERENCES users(id),
    input_file_id  UUID NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    output_file_id UUID REFERENCES files(id) ON DELETE CASCADE,
    lang           VARCHAR(64) NOT NULL DEFAULT 'eng',
    status         VARCHAR(20) NOT NULL CHECK (status IN ('pending','processing','done','failed')),
    created_at     TIMESTAMP DEFAULT NOW()
);

-- Analytic: no produced file, result is the row.
CREATE TABLE IF NOT EXISTS ocr_detect_lang_jobs (
    id            UUID PRIMARY KEY,
    user_id       UUID REFERENCES users(id),
    input_file_id UUID NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    detected_lang VARCHAR(16),
    -- Tesseract OSD reports orientation/script confidence; we surface
    -- the script confidence here. Range 0..100 in OSD output but we
    -- keep enough precision for fractional reporting.
    confidence   NUMERIC(6,2),
    status       VARCHAR(20) NOT NULL CHECK (status IN ('pending','processing','done','failed')),
    created_at   TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS ocr_structured_jobs (
    id             UUID PRIMARY KEY,
    user_id        UUID REFERENCES users(id),
    input_file_id  UUID NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    output_file_id UUID REFERENCES files(id) ON DELETE CASCADE,
    lang           VARCHAR(64) NOT NULL DEFAULT 'eng',
    status         VARCHAR(20) NOT NULL CHECK (status IN ('pending','processing','done','failed')),
    created_at     TIMESTAMP DEFAULT NOW()
);
