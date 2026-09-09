ALTER TABLE documents ADD COLUMN generation uuid NOT NULL DEFAULT uuidv7()
    CONSTRAINT documents_generation_v7 CHECK (uuid_extract_version(generation) IS NOT DISTINCT FROM 7);

CREATE OR REPLACE FUNCTION bump_document_version() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    NEW.generation := OLD.generation;
    NEW.version := OLD.version + 1;
    NEW.updated_at := clock_timestamp();
    RETURN NEW;
END;
$$;
