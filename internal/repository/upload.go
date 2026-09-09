package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mrbeaver1/dock-service/internal/models"
)

type UploadRepository interface {
	Create(context.Context, models.Upload, time.Duration) error
	Schedule(context.Context, uuid.UUID) error
	Claim(context.Context) (models.Upload, bool, error)
	Complete(context.Context, models.Upload) error
	Retry(context.Context, models.Upload) error
}

type uploadRepository struct{ db *pgxpool.Pool }

func NewUploadRepository(db *pgxpool.Pool) UploadRepository { return &uploadRepository{db: db} }

func (r *uploadRepository) Create(ctx context.Context, upload models.Upload, timeout time.Duration) error {
	_, err := r.db.Exec(ctx, `INSERT INTO upload_jobs(id,owner_id,object_key,available_at)
	    VALUES($1,$2,$3,clock_timestamp()+$4::bigint*interval '1 millisecond'+interval '1 minute')`, upload.ID, upload.OwnerID, upload.ObjectKey, timeout.Milliseconds())
	if err != nil {
		return fmt.Errorf("record upload: %w", err)
	}
	return nil
}

func (r *uploadRepository) Schedule(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `UPDATE upload_jobs SET available_at=clock_timestamp()+interval '1 minute'
	    WHERE id=$1 AND cleanup_token IS NULL`, id)
	if err != nil {
		return fmt.Errorf("schedule upload cleanup: %w", err)
	}
	return nil
}

func (r *uploadRepository) Claim(ctx context.Context) (models.Upload, bool, error) {
	var upload models.Upload
	err := r.db.QueryRow(ctx, `WITH candidate AS (
	    SELECT id FROM upload_jobs WHERE available_at<=clock_timestamp()
	    ORDER BY available_at,id FOR UPDATE SKIP LOCKED LIMIT 1
	) UPDATE upload_jobs j SET cleanup_token=uuidv7(),available_at=clock_timestamp()+interval '1 minute'
	  FROM candidate c WHERE j.id=c.id RETURNING j.id,j.owner_id,j.object_key,j.cleanup_token`).
		Scan(&upload.ID, &upload.OwnerID, &upload.ObjectKey, &upload.CleanupToken)
	if errors.Is(err, pgx.ErrNoRows) {
		return upload, false, nil
	}
	if err != nil {
		return upload, false, fmt.Errorf("claim upload cleanup: %w", err)
	}
	return upload, true, nil
}

func (r *uploadRepository) Complete(ctx context.Context, upload models.Upload) error {
	_, err := r.db.Exec(ctx, `DELETE FROM upload_jobs WHERE id=$1 AND cleanup_token=$2`, upload.ID, upload.CleanupToken)
	if err != nil {
		return fmt.Errorf("complete upload cleanup: %w", err)
	}
	return nil
}

func (r *uploadRepository) Retry(ctx context.Context, upload models.Upload) error {
	_, err := r.db.Exec(ctx, `UPDATE upload_jobs SET available_at=clock_timestamp()+interval '1 minute'
	    WHERE id=$1 AND cleanup_token=$2`, upload.ID, upload.CleanupToken)
	if err != nil {
		return fmt.Errorf("retry upload cleanup: %w", err)
	}
	return nil
}
