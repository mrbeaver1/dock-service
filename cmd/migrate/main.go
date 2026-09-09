package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/mrbeaver1/dock-service/internal/config"
	"github.com/mrbeaver1/dock-service/internal/storage/db"
	"github.com/mrbeaver1/dock-service/migrations"
)

func main() {
	if err := run(); err != nil {
		slog.Error("migrate database", "error", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) != 2 || (os.Args[1] != "up" && os.Args[1] != "down") {
		return fmt.Errorf("usage: go run ./cmd/migrate up|down")
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	cfg, err := config.LoadPostgres()
	if err != nil {
		return err
	}
	pool, err := db.NewPool(ctx, cfg)
	if err != nil {
		return err
	}
	defer pool.Close()
	if os.Args[1] == "down" {
		return migrations.Down(ctx, pool)
	}
	return migrations.Up(ctx, pool)
}
