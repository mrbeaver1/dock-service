package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mrbeaver1/dock-service/internal/models"
)

type GrantRepository interface {
	Replace(ctx context.Context, documentID uuid.UUID, userIDs []uuid.UUID) error
	ListByDocument(ctx context.Context, documentID uuid.UUID) ([]models.Grant, error)
}

type grantRepository struct{ db *pgxpool.Pool }

func NewGrantRepository(db *pgxpool.Pool) GrantRepository { return &grantRepository{db: db} }

func insertGrants(ctx context.Context, tx pgx.Tx, documentID uuid.UUID, userIDs []uuid.UUID) error {
	_, err := tx.Exec(ctx, `INSERT INTO document_grants (document_id, user_id)
	    SELECT $1, user_id FROM unnest($2::uuid[]) AS ids(user_id)
	    ON CONFLICT (document_id, user_id) DO NOTHING`, documentID, userIDs)
	return err
}

func (r *grantRepository) Replace(ctx context.Context, documentID uuid.UUID, userIDs []uuid.UUID) error {
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		var id uuid.UUID
		if err := tx.QueryRow(ctx, `SELECT id FROM documents WHERE id=$1 FOR UPDATE`, documentID).Scan(&id); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM document_grants WHERE document_id=$1`, id); err != nil {
			return err
		}
		return insertGrants(ctx, tx, id, userIDs)
	})
	return queryError("replace document grants", err, models.ErrDocumentNotFound)
}

func (r *grantRepository) ListByDocument(ctx context.Context, documentID uuid.UUID) ([]models.Grant, error) {
	rows, err := r.db.Query(ctx, `SELECT id, document_id, user_id, created_at FROM document_grants WHERE document_id=$1 ORDER BY id`, documentID)
	if err != nil {
		return nil, queryError("list grants", err, models.ErrDocumentNotFound)
	}
	defer rows.Close()
	grants := make([]models.Grant, 0)
	for rows.Next() {
		var grant models.Grant
		if err := rows.Scan(&grant.ID, &grant.DocumentID, &grant.UserID, &grant.CreatedAt); err != nil {
			return nil, err
		}
		grants = append(grants, grant)
	}
	return grants, rows.Err()
}
