package cache

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"

	"github.com/mrbeaver1/dock-service/internal/config"
	goredis "github.com/redis/go-redis/v9"
)

func NewClient(ctx context.Context, cfg config.RedisConfig) (*goredis.Client, error) {
	client := goredis.NewClient(&goredis.Options{
		Addr:                  net.JoinHostPort(cfg.Host, strconv.Itoa(int(cfg.Port))),
		Password:              cfg.Password,
		ContextTimeoutEnabled: true,
	})
	if err := client.Ping(ctx).Err(); err != nil {
		pingErr := fmt.Errorf("ping Redis: %w", err)
		if closeErr := client.Close(); closeErr != nil {
			return nil, errors.Join(pingErr, fmt.Errorf("close Redis: %w", closeErr))
		}
		return nil, pingErr
	}
	return client, nil
}
