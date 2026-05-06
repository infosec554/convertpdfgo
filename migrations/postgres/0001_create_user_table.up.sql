-- ENUM Types
DO $$ BEGIN
    CREATE TYPE user_status_enum AS ENUM ('active', 'deleted', 'blocked');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

DO $$ BEGIN
    CREATE TYPE otp_status_enum AS ENUM ('unconfirmed', 'confirmed');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

-- 2. Table: users
CREATE TABLE users (
    id UUID PRIMARY KEY,
    status user_status_enum NOT NULL,
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    password VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    created_by UUID
);

-- 3. Table: otp
CREATE TABLE otp (
    id UUID PRIMARY KEY,
    email VARCHAR(255) NOT NULL,
    status otp_status_enum NOT NULL,
    code VARCHAR(6) NOT NULL,
    expires_at TIMESTAMP
);

-- 4. Table: sysusers
CREATE TABLE sysusers (
    id UUID PRIMARY KEY,
    status user_status_enum NOT NULL,
    name VARCHAR(255) NOT NULL,
    phone VARCHAR(255) UNIQUE NOT NULL,
    password VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    created_by UUID
);

-- 5. Table: roles
CREATE TABLE roles (
    id UUID PRIMARY KEY,
    status user_status_enum NOT NULL,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    created_by UUID
);

-- 6. Table: sysuser_roles (many-to-many)
CREATE TABLE sysuser_roles (
    id UUID PRIMARY KEY,
    sysuser_id UUID NOT NULL REFERENCES sysusers(id) ON DELETE CASCADE,
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE
);

--************************************************ FILE STORAGE
CREATE TABLE files (
    id UUID PRIMARY KEY,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE, -- ❗️ NOT NULL olib tashlandi
    file_name VARCHAR(255) NOT NULL,
    file_path TEXT NOT NULL,
    file_type VARCHAR(20) NOT NULL,
    file_size INTEGER NOT NULL,
    uploaded_at TIMESTAMP DEFAULT NOW()
);

-- ORGANIZE PDF
CREATE TABLE organize_jobs (
    id UUID PRIMARY KEY,
    user_id UUID REFERENCES users(id),
    input_file_id UUID NOT NULL REFERENCES files(id),
    new_order INTEGER[] NOT NULL, -- Sahifalar yangi tartibi
    output_file_id UUID REFERENCES files(id),
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMP DEFAULT NOW()
);

-- MERGE PDF
CREATE TABLE merge_jobs (
    id UUID PRIMARY KEY,
    user_id UUID REFERENCES users(id),
    output_file_id UUID REFERENCES files(id),
    status VARCHAR(20) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE merge_job_input_files (
    id UUID PRIMARY KEY,
    job_id UUID NOT NULL REFERENCES merge_jobs(id) ON DELETE CASCADE,
    file_id UUID NOT NULL REFERENCES files(id)
);

CREATE TABLE split_jobs (
    id UUID PRIMARY KEY,
    user_id UUID REFERENCES users(id),
    input_file_id UUID NOT NULL REFERENCES files(id),
    split_ranges TEXT NOT NULL,
    output_file_ids UUID[] DEFAULT ARRAY[]::UUID[],
    status VARCHAR(20) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE remove_pages_jobs (
    id UUID PRIMARY KEY,
    user_id UUID REFERENCES users(id),
    input_file_id UUID NOT NULL REFERENCES files(id),
    pages_to_remove TEXT NOT NULL,
    output_file_id UUID REFERENCES files(id),
    status VARCHAR(20) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE extract_pages_jobs (
    id UUID PRIMARY KEY,
    user_id UUID REFERENCES users(id),
    input_file_id UUID NOT NULL REFERENCES files(id),
    pages_to_extract TEXT NOT NULL,
    output_file_id UUID REFERENCES files(id),
    status VARCHAR(20) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

-- OPTIMIZE PDF
CREATE TABLE compress_jobs (
    id UUID PRIMARY KEY,
    user_id UUID REFERENCES users(id),
    input_file_id UUID NOT NULL REFERENCES files(id),
    compression VARCHAR(10),
    output_file_id UUID REFERENCES files(id),
    status VARCHAR(20) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

-- CONVERT TO PDF
CREATE TABLE jpg_to_pdf_jobs (
    id UUID PRIMARY KEY,
    user_id UUID REFERENCES users(id),
    input_file_ids UUID[] NOT NULL, -- Bir nechta rasm fayllari
    output_file_id UUID REFERENCES files(id),
    status VARCHAR(20) NOT NULL CHECK (status IN ('pending', 'processing', 'done', 'failed')),
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE word_to_pdf_jobs (
    id UUID PRIMARY KEY,
    user_id UUID REFERENCES users(id),
    input_file_id UUID NOT NULL REFERENCES files(id), -- Bitta Word fayl
    output_file_id UUID REFERENCES files(id),
    status VARCHAR(20) NOT NULL CHECK (status IN ('pending', 'processing', 'done', 'failed')),
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE pdf_to_jpg_jobs (
    id UUID PRIMARY KEY,
    user_id UUID REFERENCES users(id),
    input_file_id UUID NOT NULL REFERENCES files(id),
    output_file_ids UUID[] DEFAULT ARRAY[]::UUID[],
    zip_file_id UUID REFERENCES files(id),
    status VARCHAR(20) NOT NULL CHECK (status IN ('pending', 'processing', 'done', 'failed')),
    created_at TIMESTAMP DEFAULT NOW()
);


CREATE TABLE rotate_jobs (
    id UUID PRIMARY KEY,
    user_id UUID REFERENCES users(id),
    input_file_id UUID NOT NULL REFERENCES files(id),
    rotation_angle INTEGER NOT NULL,   -- Burilish burchagi
    pages VARCHAR NOT NULL DEFAULT 'all', -- Sahifa diapazoni
    output_file_id UUID REFERENCES files(id),  -- Chiqish fayli
    output_path VARCHAR,  -- Faylni saqlash joyi
    status VARCHAR(20) NOT NULL,  -- Holat: 'pending', 'done'
    created_at TIMESTAMP DEFAULT now() -- Yaratilgan vaqti
);


CREATE TABLE add_page_number_jobs (
  id UUID PRIMARY KEY,
  user_id UUID REFERENCES users(id),
  input_file_id UUID NOT NULL REFERENCES files(id),
  output_file_id UUID REFERENCES files(id),
  status VARCHAR(20) NOT NULL,
  first_number INT,
  page_range VARCHAR(50),
  position VARCHAR(20),
  color VARCHAR(20),
  font_size INT,
  created_at TIMESTAMP DEFAULT now()
);



CREATE TABLE watermark_jobs (
    id UUID PRIMARY KEY,
    user_id UUID REFERENCES users(id),
    input_file_id UUID NOT NULL REFERENCES files(id),
    watermark_text TEXT NOT NULL,
    position VARCHAR(20),
    output_file_id UUID REFERENCES files(id),
    status VARCHAR(20) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE crop_pdf_jobs (
    id UUID PRIMARY KEY,
    user_id UUID REFERENCES users(id),
    input_file_id UUID NOT NULL REFERENCES files(id),
    top INTEGER NOT NULL,
    bottom INTEGER NOT NULL,
    "left" INTEGER NOT NULL,
    "right" INTEGER NOT NULL,
    output_file_id UUID REFERENCES files(id),
    status VARCHAR(20) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

-- PDF SECURITY
CREATE TABLE unlock_jobs (
    id UUID PRIMARY KEY,
    user_id UUID REFERENCES users(id),
    input_file_id UUID NOT NULL REFERENCES files(id),
    output_file_id UUID REFERENCES files(id),
    status VARCHAR(20) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Logs Table
CREATE TABLE logs (
    id UUID PRIMARY KEY,
    job_id UUID,
    job_type VARCHAR(30),
    message TEXT,
    level VARCHAR(10),
    created_at TIMESTAMP DEFAULT NOW()
);

-- PDF Inspection
CREATE TABLE pdf_inspect_jobs (
    id UUID PRIMARY KEY,
    user_id UUID REFERENCES users(id),
    file_id UUID NOT NULL REFERENCES files(id),
    page_count INT,
    title TEXT,
    author TEXT,
    subject TEXT,
    keywords TEXT,
    status VARCHAR(20) NOT NULL CHECK (status IN ('pending', 'done', 'failed')),
    created_at TIMESTAMP DEFAULT NOW()
);

-- Translation Jobs
CREATE TABLE translate_jobs (
    id UUID PRIMARY KEY,
    user_id UUID REFERENCES users(id),
    input_file_id UUID NOT NULL REFERENCES files(id),
    source_lang VARCHAR(10) NOT NULL,
    target_lang VARCHAR(10) NOT NULL,
    output_file_id UUID REFERENCES files(id),
    status VARCHAR(20) NOT NULL CHECK (status IN ('pending', 'processing', 'done', 'failed')),
    created_at TIMESTAMP DEFAULT NOW()
);

-- Shared Links
CREATE TABLE shared_links (
    id UUID PRIMARY KEY,
    file_id UUID REFERENCES files(id) ON DELETE CASCADE,
    shared_token VARCHAR(255) UNIQUE NOT NULL,
    expires_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW()
);
CREATE TABLE protect_jobs (
    id UUID PRIMARY KEY,
    user_id UUID REFERENCES users(id),
    input_file_id UUID NOT NULL REFERENCES files(id),
    output_file_id UUID REFERENCES files(id),
    password TEXT NOT NULL,
    status VARCHAR(20) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);
CREATE TABLE add_background_jobs (
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


CREATE TABLE pdf_to_word_jobs (
    id UUID PRIMARY KEY,
    user_id UUID REFERENCES users(id),
    input_file_id UUID NOT NULL REFERENCES files(id),
    output_file_id UUID REFERENCES files(id),
    status VARCHAR(50) NOT NULL CHECK (
        status IN ('pending', 'processing', 'done', 'failed', 'conversion_failed')
    ),
    created_at TIMESTAMP DEFAULT NOW()
);
CREATE TABLE word_to_pdf_jobs (
    id UUID PRIMARY KEY,
    user_id UUID REFERENCES users(id),
    input_file_id UUID NOT NULL REFERENCES files(id),
    output_file_id UUID REFERENCES files(id),
    status VARCHAR(50) NOT NULL CHECK (status IN ('pending', 'processing', 'done', 'failed')),
    created_at TIMESTAMP DEFAULT NOW()
);
CREATE TABLE excel_to_pdf_jobs (
    id UUID PRIMARY KEY,
    user_id UUID REFERENCES users(id),
    input_file_id UUID NOT NULL REFERENCES files(id),
    output_file_id UUID REFERENCES files(id),
    status VARCHAR(20) NOT NULL CHECK (status IN ('pending', 'processing', 'done', 'failed')),
    created_at TIMESTAMP DEFAULT now()
);

CREATE TABLE powerpoint_to_pdf_jobs (
    id UUID PRIMARY KEY,
    user_id UUID REFERENCES users(id),
    input_file_id UUID NOT NULL REFERENCES files(id),
    output_file_id UUID REFERENCES files(id),
    status VARCHAR(20) NOT NULL CHECK (status IN ('pending', 'processing', 'done', 'failed')),
    created_at TIMESTAMP DEFAULT now()
);

CREATE TABLE html_to_pdf_jobs (
    id UUID PRIMARY KEY,
    user_id UUID,
    html_content TEXT NOT NULL,
    output_file_id UUID,
    status VARCHAR(20) NOT NULL CHECK (status IN ('pending', 'processing', 'done', 'failed')),
    created_at TIMESTAMP DEFAULT NOW()
);
