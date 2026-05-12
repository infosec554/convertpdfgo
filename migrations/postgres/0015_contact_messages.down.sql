DELETE FROM role_permissions
    WHERE permission_id IN (SELECT id FROM permissions WHERE name LIKE 'contacts:%');
DELETE FROM permissions WHERE name LIKE 'contacts:%';

DROP INDEX IF EXISTS idx_contact_replies_message;
DROP INDEX IF EXISTS idx_contact_messages_user;
DROP INDEX IF EXISTS idx_contact_messages_status_created;
DROP TABLE IF EXISTS contact_replies;
DROP TABLE IF EXISTS contact_messages;
