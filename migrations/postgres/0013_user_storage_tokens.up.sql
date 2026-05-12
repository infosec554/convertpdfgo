-- Per-user OAuth tokens for the "Save to..." export targets.
-- One row per (user, provider) — `ON CONFLICT (user_id, provider) DO UPDATE`
-- in the repo handles re-auth without bloating the table.
--
-- `expires_at` is nullable because some providers (Dropbox, historically)
-- issue tokens that don't expire. Refresh tokens are stored encrypted at
-- rest is a future improvement — for now plaintext, with file-system
-- access being the practical threat model anyway.
CREATE TABLE user_storage_tokens (
    id              UUID PRIMARY KEY,
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider        VARCHAR(32) NOT NULL,
    access_token    TEXT NOT NULL,
    refresh_token   TEXT,
    expires_at      TIMESTAMP,
    scope           TEXT,
    account_email   VARCHAR(255),
    created_at      TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, provider)
);

CREATE INDEX idx_user_storage_tokens_user ON user_storage_tokens(user_id);
