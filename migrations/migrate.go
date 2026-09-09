package migrations

import (
	"context"
	"crypto/sha256"
	"embed"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed *.sql
var scripts embed.FS

var names = [...]string{"documents", "document_generation"}

func script(version int, direction string) ([]byte, error) {
	return scripts.ReadFile(fmt.Sprintf("%06d_%s.%s.sql", version, names[version-1], direction))
}

func checksum(data []byte) string {
	value := fmt.Sprintf("%x", sha256.Sum256(data))
	switch value {
	case "7ecb2b66ad2a2991bd549e091316f599cc917fc3da695d3382fbfc4562664b0e":
		return "94f4bab0082ab490d45cd4d34b6bcbd53788b5fff7923e78121b9726a0a8123e"
	case "4b6a8c8d92610278114e6d91870d0b221dc0a4661f41ba2f3edf7b60261f3d5b":
		return "97a2501f4c62635abf42949e5d97d607e5073de21d7c7c066fe2bd7676af8c48"
	}
	return value
}

func applied(ctx context.Context, tx pgx.Tx) (int, error) {
	rows, err := tx.Query(ctx, `SELECT version, checksum FROM schema_migrations ORDER BY version`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var version int64
		var existing string
		if err := rows.Scan(&version, &existing); err != nil {
			return 0, err
		}
		if version > int64(len(names)) {
			return 0, errors.New("database schema is newer than this binary")
		}
		if version != int64(count+1) {
			return 0, errors.New("invalid migration history")
		}
		sql, err := script(int(version), "up")
		if err != nil {
			return 0, err
		}
		if checksum(sql) != existing {
			return 0, fmt.Errorf("migration %d checksum differs from source", version)
		}
		count++
	}
	return count, rows.Err()
}

func Up(ctx context.Context, pool *pgxpool.Pool) error {
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if err := lock(ctx, tx); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
            id uuid PRIMARY KEY DEFAULT uuidv7(), version bigint NOT NULL UNIQUE,
            checksum text NOT NULL, applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
			return err
		}
		count, err := applied(ctx, tx)
		if err != nil {
			return err
		}
		for version := count + 1; version <= len(names); version++ {
			sql, err := script(version, "up")
			if err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, string(sql)); err != nil {
				return fmt.Errorf("apply migration %d: %w", version, err)
			}
			if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations(version,checksum)VALUES($1,$2)`, version, checksum(sql)); err != nil {
				return err
			}
		}
		return nil
	})
}

func Down(ctx context.Context, pool *pgxpool.Pool) error {
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if err := lock(ctx, tx); err != nil {
			return err
		}
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT to_regclass('schema_migrations') IS NOT NULL`).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return nil
		}
		version, err := applied(ctx, tx)
		if err != nil || version == 0 {
			return err
		}
		sql, err := script(version, "down")
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("roll back migration %d: %w", version, err)
		}
		_, err = tx.Exec(ctx, `DELETE FROM schema_migrations WHERE version=$1`, version)
		return err
	})
}

func lock(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(723149801)`)
	return err
}
