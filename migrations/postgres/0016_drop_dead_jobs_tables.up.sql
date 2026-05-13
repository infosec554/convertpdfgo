-- 0016: Drop dead job tables that were declared in 0001 but never wired.
--
-- `translate_jobs` and `add_background_jobs` have no handlers, no services,
-- no worker tasks. The corresponding Go package (pkg/addbackground) and
-- model (api/models/addbackground.go) were deleted alongside this migration.
--
-- Down migration re-creates them in their original shape so 0001 + 0016
-- round-trip cleanly.

DROP TABLE IF EXISTS translate_jobs;
DROP TABLE IF EXISTS add_background_jobs;
