package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mrbeaver1/dock-service/internal/models"
)

type DeletionRepository interface {
	Claim(context.Context) (models.Deletion, bool, error)
	Complete(context.Context, models.Deletion) error
	Retry(context.Context, models.Deletion) error
}

type deletionRepository struct{ db *pgxpool.Pool }

func NewDeletionRepository(db *pgxpool.Pool) DeletionRepository {
	return &deletionRepository{db: db}
}

func (r *deletionRepository) Claim(ctx context.Context) (models.Deletion, bool, error) {
	var job models.Deletion
	err := r.db.QueryRow(ctx, `WITH candidate AS (
	    SELECT j.id FROM deletion_jobs j JOIN documents d ON d.id=j.id
	    WHERE j.available_at<=clock_timestamp() AND d.deleted_at IS NOT NULL
	      AND d.owner_id=j.owner_id AND d.generation=j.generation
	    ORDER BY j.available_at,j.id FOR UPDATE OF j SKIP LOCKED LIMIT 1
	) UPDATE deletion_jobs j SET cleanup_token=uuidv7(),available_at=clock_timestamp()+interval '1 minute'
	  FROM candidate c WHERE j.id=c.id
	  RETURNING j.id,j.owner_id,j.generation,j.object_key,j.version,j.cleanup_token`).
		Scan(&job.ID, &job.OwnerID, &job.Generation, &job.ObjectKey, &job.Version, &job.CleanupToken)
	if errors.Is(err, pgx.ErrNoRows) {
		return job, false, nil
	}
	if err != nil {
		return job, false, fmt.Errorf("claim document deletion: %w", err)
	}
	return job, true, nil
}

func (r *deletionRepository) Complete(ctx context.Context, job models.Deletion) error {
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		var id uuid.UUID
		err := tx.QueryRow(ctx, `SELECT id FROM documents WHERE id=$1 AND owner_id=$2
		    AND generation=$3 AND deleted_at IS NOT NULL FOR UPDATE`, job.ID, job.OwnerID, job.Generation).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `DELETE FROM deletion_jobs WHERE id=$1 AND owner_id=$2
		    AND generation=$3 AND cleanup_token=$4`, job.ID, job.OwnerID, job.Generation, job.CleanupToken)
		if err != nil || tag.RowsAffected() == 0 {
			return err
		}
		_, err = tx.Exec(ctx, `DELETE FROM documents WHERE id=$1 AND owner_id=$2 AND generation=$3`, job.ID, job.OwnerID, job.Generation)
		return err
	})
	if err != nil {
		return fmt.Errorf("complete document deletion: %w", err)
	}
	return nil
}

func (r *deletionRepository) Retry(ctx context.Context, job models.Deletion) error {
	_, err := r.db.Exec(ctx, `UPDATE deletion_jobs SET available_at=clock_timestamp()+interval '1 minute'
	    WHERE id=$1 AND owner_id=$2 AND generation=$3 AND cleanup_token=$4`, job.ID, job.OwnerID, job.Generation, job.CleanupToken)
	if err != nil {
		return fmt.Errorf("retry document deletion: %w", err)
	}
	return nil
}
