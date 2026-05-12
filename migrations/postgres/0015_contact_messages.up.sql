-- 0015: user → admin contact inbox + admin reply thread.
--
-- contact_messages   one row per inbound submission
-- contact_replies    one row per admin reply to a message (1:N)
--
-- Status starts at 'new'; flips to 'replied' the first time an
-- admin posts a reply, and 'closed' on explicit admin action. The
-- status / created_at index is what drives the dashboard list.

CREATE TABLE contact_messages (
    id          UUID PRIMARY KEY,
    user_id     UUID REFERENCES users(id) ON DELETE SET NULL,  -- NULL for guests
    name        VARCHAR(255) NOT NULL,
    email       VARCHAR(255) NOT NULL,
    subject     VARCHAR(255),
    body        TEXT NOT NULL,
    status      VARCHAR(20) NOT NULL DEFAULT 'new'
                CHECK (status IN ('new', 'replied', 'closed')),
    replied_at  TIMESTAMP,
    replied_by  UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at  TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_contact_messages_status_created
    ON contact_messages(status, created_at DESC);

CREATE INDEX idx_contact_messages_user
    ON contact_messages(user_id);

CREATE TABLE contact_replies (
    id                  UUID PRIMARY KEY,
    contact_message_id  UUID NOT NULL REFERENCES contact_messages(id) ON DELETE CASCADE,
    admin_user_id       UUID NOT NULL REFERENCES users(id) ON DELETE SET NULL,
    body                TEXT NOT NULL,
    sent_at             TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_contact_replies_message ON contact_replies(contact_message_id);

-- Permissions for the admin side. Admin role already gets the full
-- permission set via the 0014 collapse, so the GRANT below only
-- needs to register the new rows in `permissions`.
INSERT INTO permissions (name, description) VALUES
    ('contacts:read',  'View inbound contact messages'),
    ('contacts:write', 'Reply to / update / delete contact messages')
ON CONFLICT (name) DO NOTHING;

-- Grant both to the admin role.
INSERT INTO role_permissions (role, permission_id)
SELECT 'admin', id FROM permissions WHERE name IN ('contacts:read', 'contacts:write')
ON CONFLICT DO NOTHING;
