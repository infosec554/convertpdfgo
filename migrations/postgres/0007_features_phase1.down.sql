DROP TABLE IF EXISTS metadata_jobs;
DROP TABLE IF EXISTS url_to_pdf_jobs;
DROP TABLE IF EXISTS nup_jobs;
DROP TABLE IF EXISTS extract_text_jobs;

ALTER TABLE watermark_jobs
    DROP COLUMN IF EXISTS pages,
    DROP COLUMN IF EXISTS opacity,
    DROP COLUMN IF EXISTS rotation,
    DROP COLUMN IF EXISTS image_file_id,
    DROP COLUMN IF EXISTS mode;
