-- /api/pdf/sign — place a signature image at absolute coordinates on a
-- specific page of an input PDF. Width is in PDF points (1 pt = 1/72 in);
-- height is auto-derived from the signature image's aspect ratio at
-- render time so the user only positions/sizes once.
CREATE TABLE IF NOT EXISTS sign_jobs (
    id                UUID PRIMARY KEY,
    user_id           UUID REFERENCES users(id),
    input_file_id     UUID NOT NULL REFERENCES files(id),
    signature_file_id UUID NOT NULL REFERENCES files(id),
    page              INTEGER          NOT NULL DEFAULT 1,
    x                 DOUBLE PRECISION NOT NULL DEFAULT 0,
    y                 DOUBLE PRECISION NOT NULL DEFAULT 0,
    width             DOUBLE PRECISION NOT NULL DEFAULT 150,
    output_file_id    UUID REFERENCES files(id),
    status            VARCHAR(20)      NOT NULL DEFAULT 'pending',
    created_at        TIMESTAMP        NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sign_jobs_user_id ON sign_jobs(user_id);
CREATE INDEX IF NOT EXISTS idx_sign_jobs_status  ON sign_jobs(status);
