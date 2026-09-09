package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mrbeaver1/dock-service/internal/models"
)

type DocumentRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (models.Document, error)
	GetAccess(ctx context.Context, id, userID uuid.UUID) (models.DocumentAccess, error)
	GetForRead(ctx context.Context, id, userID uuid.UUID) (models.Document, bool, error)
	Create(ctx context.Context, doc models.Document, userIDs []uuid.UUID) (models.Document, error)
	Update(ctx context.Context, doc models.Document, userIDs []uuid.UUID) (models.Document, error)
	Delete(ctx context.Context, id uuid.UUID) error
	ListByOwner(ctx context.Context, ownerID uuid.UUID, limit *uint64) ([]models.Document, error)
}

type documentRepository struct{ db *pgxpool.Pool }

func NewDocumentRepository(db *pgxpool.Pool) DocumentRepository { return &documentRepository{db: db} }

const documentColumns = `d.id, d.generation, d.owner_id, d.name, d.file, d.public, d.mime,
    d.json_data, d.object_key, d.file_size, d.version, d.created_at, d.updated_at`

const documentAllowed = `(d.owner_id = $2 OR COALESCE(d.public, false) OR EXISTS (
    SELECT 1 FROM document_grants g WHERE g.document_id = d.id AND g.user_id = $2
))`

func documentTargets(d *models.Document) []any {
	return []any{&d.ID, &d.Generation, &d.OwnerID, &d.Name, &d.File, &d.Public, &d.MIME,
		&d.JSON, &d.ObjectKey, &d.FileSize, &d.Version, &d.CreatedAt, &d.UpdatedAt}
}

func (r *documentRepository) GetByID(ctx context.Context, id uuid.UUID) (models.Document, error) {
	var doc models.Document
	err := r.db.QueryRow(ctx, `SELECT `+documentColumns+` FROM documents d WHERE d.id = $1`, id).Scan(documentTargets(&doc)...)
	return doc, queryError("get document", err, models.ErrDocumentNotFound)
}

func (r *documentRepository) GetAccess(ctx context.Context, id, userID uuid.UUID) (models.DocumentAccess, error) {
	var access models.DocumentAccess
	err := r.db.QueryRow(ctx, `SELECT d.generation, d.version, `+documentAllowed+` FROM documents d WHERE d.id = $1`, id, userID).
		Scan(&access.Generation, &access.Version, &access.Allowed)
	return access, queryError("get document access", err, models.ErrDocumentNotFound)
}

func (r *documentRepository) GetForRead(ctx context.Context, id, userID uuid.UUID) (models.Document, bool, error) {
	var doc models.Document
	var allowed bool
	targets := append(documentTargets(&doc), &allowed)
	err := r.db.QueryRow(ctx, `SELECT `+documentColumns+`, `+documentAllowed+` FROM documents d WHERE d.id = $1`, id, userID).Scan(targets...)
	return doc, allowed, queryError("read document", err, models.ErrDocumentNotFound)
}

func (r *documentRepository) Create(ctx context.Context, doc models.Document, userIDs []uuid.UUID) (models.Document, error) {
	if doc.ID == uuid.Nil {
		id, err := uuid.NewV7()
		if err != nil {
			return models.Document{}, fmt.Errorf("generate document ID: %w", err)
		}
		doc.ID = id
	}
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO documents
		    (id, owner_id, name, file, public, mime, json_data, object_key, file_size)
		    VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`, doc.ID, doc.OwnerID, doc.Name, doc.File,
			doc.Public, doc.MIME, doc.JSON, doc.ObjectKey, doc.FileSize)
		if err != nil {
			return err
		}
		if err := insertGrants(ctx, tx, doc.ID, userIDs); err != nil {
			return err
		}
		return tx.QueryRow(ctx, `SELECT `+documentColumns+` FROM documents d WHERE d.id=$1`, doc.ID).Scan(documentTargets(&doc)...)
	})
	return doc, queryError("create document", err, models.ErrDocumentNotFound)
}

func (r *documentRepository) Update(ctx context.Context, doc models.Document, userIDs []uuid.UUID) (models.Document, error) {
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE documents SET name=$2, file=$3, public=$4, mime=$5,
		    json_data=$6, object_key=$7, file_size=$8 WHERE id=$1`, doc.ID, doc.Name, doc.File,
			doc.Public, doc.MIME, doc.JSON, doc.ObjectKey, doc.FileSize)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return models.ErrDocumentNotFound
		}
		if _, err := tx.Exec(ctx, `DELETE FROM document_grants WHERE document_id=$1`, doc.ID); err != nil {
			return err
		}
		if err := insertGrants(ctx, tx, doc.ID, userIDs); err != nil {
			return err
		}
		return tx.QueryRow(ctx, `SELECT `+documentColumns+` FROM documents d WHERE d.id=$1`, doc.ID).Scan(documentTargets(&doc)...)
	})
	return doc, queryError("update document", err, models.ErrDocumentNotFound)
}

func (r *documentRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM documents WHERE id=$1`, id)
	if err != nil {
		return queryError("delete document", err, models.ErrDocumentNotFound)
	}
	if tag.RowsAffected() == 0 {
		return models.ErrDocumentNotFound
	}
	return nil
}

func (r *documentRepository) ListByOwner(ctx context.Context, ownerID uuid.UUID, limit *uint64) ([]models.Document, error) {
	query := `SELECT ` + documentColumns + ` FROM documents d WHERE d.owner_id=$1 ORDER BY d.name, d.created_at, d.id`
	args := []any{ownerID}
	if limit != nil {
		if *limit <= uint64(1<<63-1) {
			query += ` LIMIT $2`
			args = append(args, int64(*limit))
		}
	}
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, queryError("list documents", err, models.ErrDocumentNotFound)
	}
	defer rows.Close()
	docs := make([]models.Document, 0)
	for rows.Next() {
		var doc models.Document
		if err := rows.Scan(documentTargets(&doc)...); err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}
	return docs, rows.Err()
}
