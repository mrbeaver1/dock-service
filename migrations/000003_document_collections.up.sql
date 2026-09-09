ALTER TABLE users ADD COLUMN documents_revision uuid NOT NULL DEFAULT uuidv7()
    CONSTRAINT users_documents_revision_v7 CHECK (uuid_extract_version(documents_revision) IS NOT DISTINCT FROM 7);

CREATE FUNCTION bump_document_collection() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        UPDATE users SET documents_revision=uuidv7() WHERE id=OLD.owner_id;
        RETURN OLD;
    END IF;
    UPDATE users SET documents_revision=uuidv7() WHERE id=NEW.owner_id;
    IF TG_OP = 'UPDATE' AND OLD.owner_id IS DISTINCT FROM NEW.owner_id THEN
        UPDATE users SET documents_revision=uuidv7() WHERE id=OLD.owner_id;
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER documents_collection AFTER INSERT OR UPDATE OR DELETE ON documents
    FOR EACH ROW EXECUTE FUNCTION bump_document_collection();

CREATE FUNCTION refresh_grant_login_collections() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    UPDATE users SET documents_revision=uuidv7() WHERE id IN (
        SELECT d.owner_id FROM document_grants g JOIN documents d ON d.id=g.document_id
        WHERE g.user_id=NEW.id
    );
    RETURN NEW;
END;
$$;
CREATE TRIGGER users_grant_login AFTER UPDATE OF login ON users
    FOR EACH ROW WHEN (OLD.login IS DISTINCT FROM NEW.login)
    EXECUTE FUNCTION refresh_grant_login_collections();

DROP INDEX documents_owner_list_idx;
DROP INDEX documents_public_list_idx;
CREATE INDEX documents_owner_list_idx ON documents(owner_id, created_at, id);
CREATE INDEX documents_public_list_idx ON documents(owner_id, created_at, id) WHERE public IS TRUE;
