-- 0014: Two-tier role model + invite-driven admin onboarding.
--
-- Product decision: only `user` and `admin` ever exist. `moderator`
-- and `superadmin` collapse into `admin`. The sysusers table — a
-- phone-based parallel user model that nobody wired up — gets dropped
-- entirely so future contributors don't waste cycles wondering which
-- column to authenticate against.
--
-- Adds `pending` to user_status_enum so invited admins exist as DB
-- rows before they accept and set a password.

-- 1. user_status_enum gains `pending` so invited rows have a state
--    that's neither active nor blocked. Existing rows untouched.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_enum
        WHERE enumlabel = 'pending'
          AND enumtypid = 'user_status_enum'::regtype
    ) THEN
        ALTER TYPE user_status_enum ADD VALUE 'pending';
    END IF;
END $$;

-- 2. Collapse roles. Any moderator/superadmin gets re-tagged as admin.
--    permission grants follow: we replace the admin row's grants with
--    the union of (admin ∪ moderator ∪ superadmin) — effectively
--    superadmin's set, which already includes everything.
UPDATE users SET role = 'admin' WHERE role IN ('moderator', 'superadmin');

DELETE FROM role_permissions WHERE role IN ('moderator', 'superadmin');

-- Re-grant admin: full set (was previously a subset; bring it up to
-- what superadmin had so the collapse doesn't lose capability).
DELETE FROM role_permissions WHERE role = 'admin';
WITH p AS (SELECT id FROM permissions)
INSERT INTO role_permissions (role, permission_id)
SELECT 'admin', p.id FROM p
ON CONFLICT DO NOTHING;

-- 3. Drop the unused sysusers parallel auth model. Cascades wipe
--    sysuser_roles automatically.
DROP TABLE IF EXISTS sysuser_roles;
DROP TABLE IF EXISTS sysusers;

-- 4. sysusers:* permissions become dead now too — purge.
DELETE FROM role_permissions
WHERE permission_id IN (SELECT id FROM permissions WHERE name LIKE 'sysusers:%');
DELETE FROM permissions WHERE name LIKE 'sysusers:%';
