-- Audit trail for auth events. Useful for debugging "why am I locked out"
-- and for security postmortems. Keep retention at app level — see cleaner.
CREATE TABLE IF NOT EXISTS auth_audit (
    id           UUID PRIMARY KEY,
    user_id      UUID,
    email        VARCHAR(255),
    event        VARCHAR(40)  NOT NULL,  -- login_ok, login_fail, logout, password_change, password_reset, otp_login_ok
    provider     VARCHAR(20),            -- email, google, github, otp
    ip           VARCHAR(45),
    user_agent   TEXT,
    created_at   TIMESTAMP    NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_auth_audit_user_id   ON auth_audit (user_id);
CREATE INDEX IF NOT EXISTS idx_auth_audit_email     ON auth_audit (email);
CREATE INDEX IF NOT EXISTS idx_auth_audit_event     ON auth_audit (event);
CREATE INDEX IF NOT EXISTS idx_auth_audit_created   ON auth_audit (created_at);
