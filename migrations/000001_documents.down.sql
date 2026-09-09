DROP TRIGGER document_grants_version ON document_grants;
DROP FUNCTION bump_granted_document_version();
DROP TABLE document_grants;
DROP TRIGGER documents_version ON documents;
DROP FUNCTION bump_document_version();
DROP TABLE documents;
DROP TABLE users;
