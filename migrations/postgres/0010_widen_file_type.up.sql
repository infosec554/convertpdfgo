-- The files.file_type column was VARCHAR(20) — far too tight for real MIME
-- types (e.g. application/vnd.openxmlformats-officedocument.wordprocessingml.document
-- is 78 chars). pdf-to-word and the other Office converters were failing with
-- "value too long" the moment they saved output rows. Widen to TEXT; we never
-- index this column so there's no downside.

ALTER TABLE files ALTER COLUMN file_type TYPE TEXT;
