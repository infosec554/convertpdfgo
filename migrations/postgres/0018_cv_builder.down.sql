-- 0018_cv_builder.down.sql
DROP TABLE IF EXISTS cv_rate_limits;
DROP TABLE IF EXISTS cv_voice_segments;
DROP TABLE IF EXISTS cv_ai_history;
DROP TABLE IF EXISTS cv_ats_runs;
DROP TABLE IF EXISTS cv_templates;
DROP TABLE IF EXISTS cv_sections;
DROP TABLE IF EXISTS cv_sessions;
-- vector extension stays — used by 0017 (PDF Chat AI).
