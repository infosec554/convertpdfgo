-- AI-generated Excel builder jobs. Sibling of pptx_builder_jobs —
-- topic + tone + language drives a Gemini call that returns a sheet
-- spec (headers, rows, formulas, optional chart). A python xlsxwriter
-- script renders it to a real .xlsx with charts and number formats.
CREATE TABLE IF NOT EXISTS xlsx_builder_jobs (
    id              UUID PRIMARY KEY,
    user_id         UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    topic           VARCHAR(500)  NOT NULL,
    sheet_count     INT           NOT NULL DEFAULT 2,
    tone            VARCHAR(32)   NOT NULL DEFAULT 'corporate',
    language        VARCHAR(8)    NOT NULL DEFAULT 'en',
    audience        VARCHAR(200)  NOT NULL DEFAULT '',
    include_chart   BOOLEAN       NOT NULL DEFAULT true,
    status          VARCHAR(16)   NOT NULL DEFAULT 'pending',
    error           TEXT          NOT NULL DEFAULT '',
    output_file_id  UUID NULL REFERENCES files(id) ON DELETE SET NULL,
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS xlsx_builder_jobs_status_idx ON xlsx_builder_jobs(status);
CREATE INDEX IF NOT EXISTS xlsx_builder_jobs_user_idx   ON xlsx_builder_jobs(user_id);
