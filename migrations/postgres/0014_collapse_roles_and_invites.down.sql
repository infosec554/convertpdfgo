-- Re-create sysusers (data is irrecoverable — the down only restores
-- the schema so a re-up doesn't fail).
CREATE TABLE IF NOT EXISTS sysusers (
    id UUID PRIMARY KEY,
    status user_status_enum NOT NULL,
    name VARCHAR(255) NOT NULL,
    phone VARCHAR(255) UNIQUE NOT NULL,
    password VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    created_by UUID
);
CREATE TABLE IF NOT EXISTS sysuser_roles (
    id UUID PRIMARY KEY,
    sysuser_id UUID NOT NULL REFERENCES sysusers(id) ON DELETE CASCADE,
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE
);

-- Reseed the perms / role grants the 0014 migration purged.
INSERT INTO permissions (name, description) VALUES
    ('sysusers:read',  'View sysusers'),
    ('sysusers:write', 'Create/edit sysusers')
ON CONFLICT (name) DO NOTHING;

-- Note: we can't re-introduce 'moderator' / 'superadmin' as user
-- rows; collapsing was destructive on purpose. The down migration
-- intentionally doesn't try.
