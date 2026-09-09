package repository

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/mrbeaver1/dock-service/internal/models"
)

func queryError(operation string, err, notFound error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%s: %w", operation, notFound)
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return fmt.Errorf("%s: %w: %w", operation, models.ErrConflict, err)
		case "22003", "22021", "22P02", "22P05", "23503", "23514":
			return fmt.Errorf("%s: %w: %w", operation, models.ErrInvalidData, err)
		}
	}
	return fmt.Errorf("%s: %w", operation, err)
}
