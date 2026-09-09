DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM deletion_jobs) THEN
        RAISE EXCEPTION 'cannot remove deletion recovery while jobs are pending';
    END IF;
END;
$$;

DROP TRIGGER documents_deletion ON documents;
DROP FUNCTION schedule_document_deletion();
DROP TABLE deletion_jobs;
