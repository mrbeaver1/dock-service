CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    login text NOT NULL UNIQUE,
    password_hash text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT users_id_v7 CHECK (uuid_extract_version(id) IS NOT DISTINCT FROM 7)
);

CREATE TABLE documents (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    owner_id uuid NOT NULL REFERENCES users(id),
    name text,
    file boolean,
    public boolean,
    mime text,
    json_data jsonb,
    object_key text UNIQUE,
    file_size bigint,
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT documents_id_v7 CHECK (uuid_extract_version(id) IS NOT DISTINCT FROM 7),
    CONSTRAINT document_object_metadata CHECK (
        (object_key IS NULL AND file_size IS NULL) OR
        (object_key IS NOT NULL AND file_size IS NOT NULL AND file_size >= 0)
    )
);

CREATE TABLE document_grants (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    document_id uuid NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (document_id, user_id),
    CONSTRAINT grants_id_v7 CHECK (uuid_extract_version(id) IS NOT DISTINCT FROM 7)
);

CREATE INDEX documents_owner_list_idx ON documents(owner_id, name, created_at, id);
CREATE INDEX documents_public_list_idx ON documents(name, created_at, id) WHERE public IS TRUE;
CREATE INDEX grants_user_document_idx ON document_grants(user_id, document_id);

CREATE FUNCTION bump_document_version() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    NEW.version := OLD.version + 1;
    NEW.updated_at := clock_timestamp();
    RETURN NEW;
END;
$$;
CREATE TRIGGER documents_version BEFORE UPDATE ON documents
    FOR EACH ROW EXECUTE FUNCTION bump_document_version();

CREATE FUNCTION bump_granted_document_version() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        UPDATE documents SET updated_at = clock_timestamp() WHERE id = OLD.document_id;
        RETURN OLD;
    END IF;
    UPDATE documents SET updated_at = clock_timestamp() WHERE id = NEW.document_id;
    IF TG_OP = 'UPDATE' AND OLD.document_id IS DISTINCT FROM NEW.document_id THEN
        UPDATE documents SET updated_at = clock_timestamp() WHERE id = OLD.document_id;
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER document_grants_version AFTER INSERT OR UPDATE OR DELETE ON document_grants
    FOR EACH ROW EXECUTE FUNCTION bump_granted_document_version();
