package db

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mrbeaver1/dock-service/internal/config"
)

func NewPool(ctx context.Context, cfg config.PostgresConfig) (*pgxpool.Pool, error) {
	poolConfig, err := parseConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("configure PostgreSQL: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create PostgreSQL pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping PostgreSQL: %w", err)
	}
	return pool, nil
}

func parseConfig(cfg config.PostgresConfig) (*pgxpool.Config, error) {
	sslMode := cfg.SSLMode
	if sslMode == "" {
		sslMode = "prefer"
	}
	// URL encoding preserves special characters in credentials and database names.
	dsn := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(cfg.User, cfg.Password),
		Host:   net.JoinHostPort(cfg.Host, strconv.Itoa(int(cfg.Port))),
		Path:   "/" + cfg.Database,
		RawQuery: url.Values{
			"sslmode":          {sslMode},
			"application_name": {"dock-service"},
		}.Encode(),
	}
	return pgxpool.ParseConfig(dsn.String())
}
