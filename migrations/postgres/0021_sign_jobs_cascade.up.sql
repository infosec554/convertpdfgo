-- Auto-delete cleaner was failing every minute with FK violations from
-- sign_jobs once a sign job had run — its three file refs (input,
-- signature, output) pinned the file rows even after TTL expired.
-- Switch each FK to ON DELETE SET NULL so the file row goes away and
-- the job row stays as an audit record with null fields.

ALTER TABLE sign_jobs
    DROP CONSTRAINT IF EXISTS sign_jobs_input_file_id_fkey,
    DROP CONSTRAINT IF EXISTS sign_jobs_signature_file_id_fkey,
    DROP CONSTRAINT IF EXISTS sign_jobs_output_file_id_fkey;

-- Re-add the columns as nullable + FK ON DELETE SET NULL.
ALTER TABLE sign_jobs
    ALTER COLUMN input_file_id     DROP NOT NULL,
    ALTER COLUMN signature_file_id DROP NOT NULL;

ALTER TABLE sign_jobs
    ADD CONSTRAINT sign_jobs_input_file_id_fkey
        FOREIGN KEY (input_file_id)     REFERENCES files(id) ON DELETE SET NULL,
    ADD CONSTRAINT sign_jobs_signature_file_id_fkey
        FOREIGN KEY (signature_file_id) REFERENCES files(id) ON DELETE SET NULL,
    ADD CONSTRAINT sign_jobs_output_file_id_fkey
        FOREIGN KEY (output_file_id)    REFERENCES files(id) ON DELETE SET NULL;
