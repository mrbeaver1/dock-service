DROP TRIGGER users_grant_login ON users;
DROP FUNCTION refresh_grant_login_collections();
DROP TRIGGER documents_collection ON documents;
DROP FUNCTION bump_document_collection();
ALTER TABLE users DROP COLUMN documents_revision;
