package main

import (
	"context"
	"log/slog"
	"os/signal"
	"syscall"

	"github.com/mrbeaver1/dock-service/internal/app"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	app, err := app.New(ctx)

	defer func() {
		if app != nil {
			err := app.Close()

			if err != nil {
				slog.Error(err.Error())
			}
			return
		}
	}()

	if err != nil {
		slog.Error(err.Error())
		return
	}

}
