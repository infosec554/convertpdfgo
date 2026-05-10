-- Partial index for the every-5-min guest-file cleaner. The unfiltered
-- idx_files_uploaded_at indexes every row, but only ~5% of files are
-- guest uploads. A partial index over the only column that matters cuts
-- both index size (~20×) and scan cost (~50×).
CREATE INDEX IF NOT EXISTS idx_files_guest_old
    ON files (uploaded_at)
    WHERE user_id IS NULL;

-- Composite for /admin/stats/storage so the GROUP BY user_id, ORDER BY
-- SUM(file_size) DESC doesn't have to sort the whole table.
CREATE INDEX IF NOT EXISTS idx_files_user_size
    ON files (user_id, file_size);

-- Approximate dashboard counters use these columns; together they cover
-- the FILTER aggregates introduced in admin_stats.UserBuckets().
CREATE INDEX IF NOT EXISTS idx_users_status ON users (status);
CREATE INDEX IF NOT EXISTS idx_users_role   ON users (role);

-- Logs table grows unbounded today. Index on created_at lets the
-- retention cron (DELETE WHERE created_at < NOW() - 30d) hit a btree
-- range scan instead of a sequential scan.
CREATE INDEX IF NOT EXISTS idx_logs_created_at ON logs (created_at);
