package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/mrbeaver1/dock-service/internal/models"
)

func (r *documentRepository) MarkDeleted(ctx context.Context, id, ownerID, generation uuid.UUID) (models.Document, error) {
	var document models.Document
	err := r.db.QueryRow(ctx, `UPDATE documents d SET deleted_at=COALESCE(deleted_at,clock_timestamp())
	    WHERE id=$1 AND owner_id=$2 AND generation=$3 RETURNING `+documentColumns, id, ownerID, generation).
		Scan(documentTargets(&document)...)
	return document, queryError("mark document deleted", err, models.ErrDocumentNotFound)
}

func (r *documentRepository) FinishDelete(ctx context.Context, id, ownerID, generation uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM documents
	    WHERE id=$1 AND owner_id=$2 AND generation=$3 AND deleted_at IS NOT NULL`, id, ownerID, generation)
	return queryError("finish document deletion", err, models.ErrDocumentNotFound)
}
