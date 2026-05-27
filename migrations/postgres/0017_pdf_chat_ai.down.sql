-- 0017_pdf_chat_ai.down.sql
-- Reverses 0017_pdf_chat_ai.up.sql. Drop tables in reverse dependency
-- order; CASCADE handles index/constraint cleanup.

DROP TABLE IF EXISTS chat_rate_limits;
DROP TABLE IF EXISTS chat_suggestions;
DROP TABLE IF EXISTS pdf_chunks;
DROP TABLE IF EXISTS chat_messages;
DROP TABLE IF EXISTS chat_sessions;

-- We leave the `vector` extension installed — it's cheap and another
-- feature may want it later. Drop manually if you really need to.
-- DROP EXTENSION IF EXISTS vector;
