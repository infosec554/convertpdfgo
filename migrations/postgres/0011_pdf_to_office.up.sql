-- Three "PDF → Office/HTML" job tables. All follow the same shape as
-- pdf_to_word_jobs: one input PDF, one output file row, status field
-- with the same allowed values. Output FK is ON DELETE CASCADE so the
-- 5-min privacy cleaner takes the job row down with the file.

CREATE TABLE IF NOT EXISTS pdf_to_html_jobs (
    id             UUID PRIMARY KEY,
    user_id        UUID REFERENCES users(id),
    input_file_id  UUID NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    output_file_id UUID REFERENCES files(id) ON DELETE CASCADE,
    status         VARCHAR(20) NOT NULL CHECK (
        status IN ('pending', 'processing', 'done', 'failed')
    ),
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS pdf_to_ppt_jobs (
    id             UUID PRIMARY KEY,
    user_id        UUID REFERENCES users(id),
    input_file_id  UUID NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    output_file_id UUID REFERENCES files(id) ON DELETE CASCADE,
    status         VARCHAR(20) NOT NULL CHECK (
        status IN ('pending', 'processing', 'done', 'failed')
    ),
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS pdf_to_excel_jobs (
    id             UUID PRIMARY KEY,
    user_id        UUID REFERENCES users(id),
    input_file_id  UUID NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    output_file_id UUID REFERENCES files(id) ON DELETE CASCADE,
    status         VARCHAR(20) NOT NULL CHECK (
        status IN ('pending', 'processing', 'done', 'failed')
    ),
    created_at TIMESTAMP DEFAULT NOW()
);
