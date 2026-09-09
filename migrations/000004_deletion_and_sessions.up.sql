ALTER TABLE documents ADD COLUMN deleted_at timestamptz;

CREATE TABLE revoked_sessions (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    token_hash bytea NOT NULL UNIQUE CHECK (octet_length(token_hash)=32),
    expires_at timestamptz,
    CONSTRAINT revoked_sessions_id_v7 CHECK (uuid_extract_version(id) IS NOT DISTINCT FROM 7)
);
CREATE INDEX revoked_sessions_expiry_idx ON revoked_sessions(expires_at);
