CREATE TABLE deletion_jobs (
    id uuid PRIMARY KEY REFERENCES documents(id) ON DELETE CASCADE,
    owner_id uuid NOT NULL,
    generation uuid NOT NULL,
    object_key text,
    version bigint NOT NULL CHECK (version > 0),
    available_at timestamptz NOT NULL,
    cleanup_token uuid,
    CONSTRAINT deletion_jobs_id_v7 CHECK (uuid_extract_version(id) IS NOT DISTINCT FROM 7),
    CONSTRAINT deletion_jobs_generation_v7 CHECK (uuid_extract_version(generation) IS NOT DISTINCT FROM 7),
    CONSTRAINT deletion_jobs_token_v7 CHECK (cleanup_token IS NULL OR uuid_extract_version(cleanup_token) IS NOT DISTINCT FROM 7)
);
CREATE INDEX deletion_jobs_available_idx ON deletion_jobs(available_at,id);

CREATE FUNCTION schedule_document_deletion() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    INSERT INTO deletion_jobs(id,owner_id,generation,object_key,version,available_at)
    VALUES(NEW.id,NEW.owner_id,NEW.generation,NEW.object_key,OLD.version,clock_timestamp()+interval '1 minute');
    RETURN NEW;
END;
$$;
CREATE TRIGGER documents_deletion AFTER UPDATE OF deleted_at ON documents
    FOR EACH ROW WHEN (OLD.deleted_at IS NULL AND NEW.deleted_at IS NOT NULL)
    EXECUTE FUNCTION schedule_document_deletion();

INSERT INTO deletion_jobs(id,owner_id,generation,object_key,version,available_at)
SELECT id,owner_id,generation,object_key,version,clock_timestamp()
FROM documents WHERE deleted_at IS NOT NULL;
