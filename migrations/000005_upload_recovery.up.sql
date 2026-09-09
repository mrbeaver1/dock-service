CREATE TABLE upload_jobs (
    id uuid PRIMARY KEY,
    owner_id uuid NOT NULL,
    object_key text NOT NULL UNIQUE,
    available_at timestamptz NOT NULL,
    cleanup_token uuid,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    CONSTRAINT upload_jobs_id_v7 CHECK (uuid_extract_version(id) IS NOT DISTINCT FROM 7),
    CONSTRAINT upload_jobs_token_v7 CHECK (cleanup_token IS NULL OR uuid_extract_version(cleanup_token) IS NOT DISTINCT FROM 7),
    CONSTRAINT upload_jobs_object_key CHECK (object_key = 'documents/' || id::text)
);
CREATE INDEX upload_jobs_available_idx ON upload_jobs(available_at,id);
