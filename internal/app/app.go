package app

import (
	"context"
	"fmt"
	"net/http"
)

type App struct {
	diContainer *diContainer
	httpServer  *http.Server
}

func New(ctx context.Context) (*App, error) {
	di, err := newDiContainer(ctx)
	if err != nil {
		return nil, fmt.Errorf("initialize application: %w", err)
	}

	return &App{diContainer: di}, nil
}

func (a *App) Close() error {
	return a.diContainer.Close()
}
