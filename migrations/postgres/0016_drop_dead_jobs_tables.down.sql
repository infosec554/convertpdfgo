-- Down: re-create the dead tables in their original 0001 shape.

CREATE TABLE IF NOT EXISTS translate_jobs (
    id UUID PRIMARY KEY,
    user_id UUID REFERENCES users(id),
    input_file_id UUID NOT NULL REFERENCES files(id),
    source_lang VARCHAR(10) NOT NULL,
    target_lang VARCHAR(10) NOT NULL,
    output_file_id UUID REFERENCES files(id),
    status VARCHAR(20) NOT NULL CHECK (status IN ('pending', 'processing', 'done', 'failed')),
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS add_background_jobs (
    id UUID PRIMARY KEY,
    user_id UUID REFERENCES users(id),
    input_file_id UUID NOT NULL REFERENCES files(id),
    background_image_file_id UUID NOT NULL REFERENCES files(id),
    opacity INTEGER NOT NULL,
    position VARCHAR(50) NOT NULL,
    output_file_id UUID REFERENCES files(id),
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMP DEFAULT NOW()
);
