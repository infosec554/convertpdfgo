ALTER TABLE sign_jobs
    DROP CONSTRAINT IF EXISTS sign_jobs_input_file_id_fkey,
    DROP CONSTRAINT IF EXISTS sign_jobs_signature_file_id_fkey,
    DROP CONSTRAINT IF EXISTS sign_jobs_output_file_id_fkey;

ALTER TABLE sign_jobs
    ADD CONSTRAINT sign_jobs_input_file_id_fkey
        FOREIGN KEY (input_file_id)     REFERENCES files(id),
    ADD CONSTRAINT sign_jobs_signature_file_id_fkey
        FOREIGN KEY (signature_file_id) REFERENCES files(id),
    ADD CONSTRAINT sign_jobs_output_file_id_fkey
        FOREIGN KEY (output_file_id)    REFERENCES files(id);
