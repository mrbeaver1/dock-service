package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/mrbeaver1/dock-service/internal/models"
)

func (r *documentRepository) GetCollection(ctx context.Context, ownerID uuid.UUID, login *string) (models.DocumentCollection, error) {
	query := `SELECT id, documents_revision FROM users WHERE id=$1`
	var value any = ownerID
	if login != nil {
		query = `SELECT id, documents_revision FROM users WHERE login=$1`
		value = *login
	}
	var collection models.DocumentCollection
	err := r.db.QueryRow(ctx, query, value).Scan(&collection.OwnerID, &collection.Revision)
	return collection, queryError("get document collection", err, models.ErrUserNotFound)
}

func (r *documentRepository) List(ctx context.Context, options models.DocumentListQuery) ([]models.DocumentListItem, error) {
	query := `SELECT d.id, d.name, d.mime, d.file, d.public, d.created_at,
        ARRAY(SELECT u.login FROM document_grants g JOIN users u ON u.id=g.user_id
            WHERE g.document_id=d.id ORDER BY u.login)
        FROM documents d WHERE d.owner_id=$1 AND d.deleted_at IS NULL AND ` + documentAllowed
	args := []any{options.OwnerID, options.ViewerID}
	if filter := options.Filter; filter != nil {
		switch filter.Field {
		case "id":
			query += ` AND d.id=$3`
			args = append(args, filter.ID)
		case "name":
			query += ` AND d.name=$3`
			args = append(args, filter.Text)
		case "mime":
			query += ` AND d.mime=$3`
			args = append(args, filter.Text)
		case "file":
			query += ` AND d.file=$3`
			args = append(args, filter.Bool)
		case "public":
			query += ` AND d.public=$3`
			args = append(args, filter.Bool)
		case "created":
			query += ` AND d.created_at >= $3 AND d.created_at < $4`
			args = append(args, filter.From, filter.Until)
		case "grant":
			query += ` AND EXISTS (SELECT 1 FROM document_grants g JOIN users u ON u.id=g.user_id
                WHERE g.document_id=d.id AND u.login=$3)`
			args = append(args, filter.Text)
		default:
			return nil, models.ErrInvalidData
		}
	}
	query += ` ORDER BY d.name ASC NULLS LAST, d.created_at, d.id`
	if options.Limit != nil && *options.Limit <= uint64(1<<63-1) {
		args = append(args, int64(*options.Limit))
		query += fmt.Sprintf(" LIMIT $%d", len(args))
	}
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, queryError("list documents", err, models.ErrDocumentNotFound)
	}
	defer rows.Close()
	documents := make([]models.DocumentListItem, 0)
	for rows.Next() {
		var document models.DocumentListItem
		if err := rows.Scan(&document.ID, &document.Name, &document.MIME, &document.File,
			&document.Public, &document.CreatedAt, &document.Grant); err != nil {
			return nil, err
		}
		documents = append(documents, document)
	}
	return documents, rows.Err()
}
