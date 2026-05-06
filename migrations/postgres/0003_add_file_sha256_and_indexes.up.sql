-- Add SHA-256 column to files for deduplication and result caching.
ALTER TABLE files ADD COLUMN IF NOT EXISTS sha256 CHAR(64);

-- Lookup an existing upload by its hash before saving a duplicate.
CREATE INDEX IF NOT EXISTS idx_files_sha256 ON files (sha256) WHERE sha256 IS NOT NULL;

-- Hot paths frequently read by user_id / created_at.
CREATE INDEX IF NOT EXISTS idx_files_user_id ON files (user_id);
CREATE INDEX IF NOT EXISTS idx_files_uploaded_at ON files (uploaded_at);

-- Job tables: indices on user_id and status keep the dashboard fast as the
-- jobs table grows.
CREATE INDEX IF NOT EXISTS idx_merge_jobs_user_id ON merge_jobs (user_id);
CREATE INDEX IF NOT EXISTS idx_merge_jobs_status ON merge_jobs (status);

CREATE INDEX IF NOT EXISTS idx_compress_jobs_user_id ON compress_jobs (user_id);
CREATE INDEX IF NOT EXISTS idx_compress_jobs_status ON compress_jobs (status);

CREATE INDEX IF NOT EXISTS idx_split_jobs_user_id ON split_jobs (user_id);
CREATE INDEX IF NOT EXISTS idx_split_jobs_status ON split_jobs (status);
