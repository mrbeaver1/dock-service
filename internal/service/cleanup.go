package service

import (
	"context"
	"log/slog"
	"time"
)

func runCleanup(ctx context.Context, name string, reconcile func(context.Context) (bool, error)) {
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		for i := 0; i < 16 && ctx.Err() == nil; i++ {
			found, err := reconcile(ctx)
			if err != nil && ctx.Err() == nil {
				slog.ErrorContext(ctx, "reconcile "+name, "error", err)
			}
			if !found {
				break
			}
		}
		timer.Reset(10 * time.Second)
	}
}
