-- AI-generated PowerPoint builder jobs.
-- Topic + tone + language drives a Gemini call that returns a slide
-- outline JSON, which a python-pptx script renders to a real PPTX.
CREATE TABLE IF NOT EXISTS pptx_builder_jobs (
    id              UUID PRIMARY KEY,
    user_id         UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    topic           VARCHAR(500)  NOT NULL,
    slide_count     INT           NOT NULL DEFAULT 8,
    tone            VARCHAR(32)   NOT NULL DEFAULT 'corporate',
    language        VARCHAR(8)    NOT NULL DEFAULT 'en',
    audience        VARCHAR(200)  NOT NULL DEFAULT '',
    status          VARCHAR(16)   NOT NULL DEFAULT 'pending',
    error           TEXT          NOT NULL DEFAULT '',
    output_file_id  UUID NULL REFERENCES files(id) ON DELETE SET NULL,
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS pptx_builder_jobs_status_idx ON pptx_builder_jobs(status);
CREATE INDEX IF NOT EXISTS pptx_builder_jobs_user_idx   ON pptx_builder_jobs(user_id);
