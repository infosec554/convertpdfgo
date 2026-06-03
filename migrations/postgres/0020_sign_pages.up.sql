-- Multi-page signing: replace the single-integer `page` column with a
-- pdfcpu-style selector string ("all" / "1,3,5" / "even" / "1-3") so
-- the same signature can land on multiple pages in one job.
ALTER TABLE sign_jobs
    ADD COLUMN IF NOT EXISTS pages VARCHAR(64) NOT NULL DEFAULT 'all';

-- Backfill `pages` from the old `page` column where it had data.
UPDATE sign_jobs
   SET pages = page::text
 WHERE pages = 'all' AND page IS NOT NULL;

ALTER TABLE sign_jobs DROP COLUMN IF EXISTS page;
