DROP INDEX IF EXISTS idx_split_jobs_status;
DROP INDEX IF EXISTS idx_split_jobs_user_id;
DROP INDEX IF EXISTS idx_compress_jobs_status;
DROP INDEX IF EXISTS idx_compress_jobs_user_id;
DROP INDEX IF EXISTS idx_merge_jobs_status;
DROP INDEX IF EXISTS idx_merge_jobs_user_id;
DROP INDEX IF EXISTS idx_files_uploaded_at;
DROP INDEX IF EXISTS idx_files_user_id;
DROP INDEX IF EXISTS idx_files_sha256;

ALTER TABLE files DROP COLUMN IF EXISTS sha256;
