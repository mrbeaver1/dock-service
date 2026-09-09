package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mrbeaver1/dock-service/internal/models"
)

type SessionRepository interface {
	IsRevoked(context.Context, [32]byte) (bool, error)
	Revoke(context.Context, models.Session) error
}

type sessionRepository struct{ db *pgxpool.Pool }

func NewSessionRepository(db *pgxpool.Pool) SessionRepository { return &sessionRepository{db: db} }

func (r *sessionRepository) IsRevoked(ctx context.Context, hash [32]byte) (bool, error) {
	var revoked bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM revoked_sessions WHERE token_hash=$1)`, hash[:]).Scan(&revoked)
	if err != nil {
		return false, fmt.Errorf("check revoked session: %w", err)
	}
	return revoked, nil
}

func (r *sessionRepository) Revoke(ctx context.Context, session models.Session) error {
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM revoked_sessions WHERE expires_at <= $1`, time.Now()); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO revoked_sessions(token_hash, expires_at) VALUES($1,$2)
		    ON CONFLICT (token_hash) DO NOTHING`, session.TokenHash[:], session.ExpiresAt)
		return err
	})
	if err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	return nil
}
