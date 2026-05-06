-- ===================================================================
-- Admin RBAC: role on users, permissions catalog, role↔permission map.
-- The existing `roles` table only powers sysusers; for app-level RBAC
-- we keep things simpler with a string column on users + a permission
-- catalog seeded below.
-- ===================================================================

-- 1. Add role column to users.  'user' is the default for fresh signups
--    so existing rows keep working without backfill.
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS role VARCHAR(20) NOT NULL DEFAULT 'user';

CREATE INDEX IF NOT EXISTS idx_users_role ON users (role);

-- 2. Permission catalog. `name` is the wire format (e.g. "users:read")
--    so handlers can RequirePermission("users:read") without hitting any
--    enum table.
CREATE TABLE IF NOT EXISTS permissions (
    id          SERIAL PRIMARY KEY,
    name        VARCHAR(64) UNIQUE NOT NULL,
    description TEXT,
    created_at  TIMESTAMP NOT NULL DEFAULT NOW()
);

-- 3. Many-to-many between role names and permissions.
--    Storing role as a string (not FK) keeps roles free-form and avoids
--    a giant join when 99% of requests just need "what can role X do".
CREATE TABLE IF NOT EXISTS role_permissions (
    role          VARCHAR(20) NOT NULL,
    permission_id INTEGER     NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role, permission_id)
);

CREATE INDEX IF NOT EXISTS idx_role_permissions_role ON role_permissions (role);

-- 4. Seed permissions.
INSERT INTO permissions (name, description) VALUES
    ('users:read',     'View users'),
    ('users:write',    'Edit users (role, status)'),
    ('users:delete',   'Delete users (soft)'),
    ('sysusers:read',  'View sysusers'),
    ('sysusers:write', 'Create/edit sysusers'),
    ('roles:read',     'View roles'),
    ('roles:write',    'Create/edit roles'),
    ('jobs:read',      'View all jobs'),
    ('jobs:moderate',  'Retry/delete any job'),
    ('files:read',     'View any file'),
    ('files:delete',   'Delete any file'),
    ('audit:read',     'View audit log'),
    ('stats:read',     'View system stats')
ON CONFLICT (name) DO NOTHING;

-- 5. Seed role → permission grants.
--    moderator: read + moderate jobs + read audit/files
--    admin:     full app-level admin (no sysuser/role mgmt)
--    superadmin: everything
WITH p AS (SELECT id, name FROM permissions)
INSERT INTO role_permissions (role, permission_id)
SELECT 'moderator', p.id FROM p WHERE p.name IN
    ('jobs:read','jobs:moderate','files:read','audit:read','stats:read')
ON CONFLICT DO NOTHING;

WITH p AS (SELECT id, name FROM permissions)
INSERT INTO role_permissions (role, permission_id)
SELECT 'admin', p.id FROM p WHERE p.name IN
    ('users:read','users:write','users:delete',
     'jobs:read','jobs:moderate',
     'files:read','files:delete',
     'audit:read','stats:read')
ON CONFLICT DO NOTHING;

WITH p AS (SELECT id FROM permissions)
INSERT INTO role_permissions (role, permission_id)
SELECT 'superadmin', p.id FROM p
ON CONFLICT DO NOTHING;
