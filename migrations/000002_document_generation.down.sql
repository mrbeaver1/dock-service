CREATE OR REPLACE FUNCTION bump_document_version() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    NEW.version := OLD.version + 1;
    NEW.updated_at := clock_timestamp();
    RETURN NEW;
END;
$$;

ALTER TABLE documents DROP COLUMN generation;
