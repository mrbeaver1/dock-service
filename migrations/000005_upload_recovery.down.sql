DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM upload_jobs) THEN
        RAISE EXCEPTION 'complete pending upload recovery before rolling back';
    END IF;
END;
$$;
DROP TABLE upload_jobs;
