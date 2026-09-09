DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM documents WHERE deleted_at IS NOT NULL) THEN
        RAISE EXCEPTION 'complete pending document deletions before rolling back';
    END IF;
END;
$$;
DROP TABLE revoked_sessions;
ALTER TABLE documents DROP COLUMN deleted_at;
