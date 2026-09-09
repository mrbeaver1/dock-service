package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type App struct {
	diContainer *diContainer
	httpServer  *http.Server
	requests    requestGroup
}

func New(ctx context.Context) (*App, error) {
	di, err := newDiContainer(ctx)
	if err != nil {
		return nil, fmt.Errorf("initialize application: %w", err)
	}

	handler, err := di.Handler()
	if err != nil {
		return nil, errors.Join(fmt.Errorf("initialize HTTP handler: %w", err), di.Close())
	}
	a := &App{diContainer: di}
	a.httpServer = newHTTPServer(ctx, net.JoinHostPort("", strconv.Itoa(int(di.Cnf().HTTPPort))), a.requests.wrap(handler))
	return a, nil
}

func newHTTPServer(ctx context.Context, address string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              address,
		Handler:           handler,
		BaseContext:       func(net.Listener) context.Context { return context.WithoutCancel(ctx) },
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       time.Minute,
	}
}

func (a *App) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() { errCh <- a.httpServer.ListenAndServe() }()
	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		shutdownErr := a.httpServer.Shutdown(shutdownCtx)
		if shutdownErr != nil {
			shutdownErr = errors.Join(shutdownErr, a.httpServer.Close())
		}
		a.requests.wait()
		serveErr := <-errCh
		if errors.Is(serveErr, http.ErrServerClosed) {
			serveErr = nil
		}
		return errors.Join(shutdownErr, serveErr)
	}
}

func (a *App) Close() error {
	serverErr := a.httpServer.Close()
	a.requests.wait()
	return errors.Join(serverErr, a.diContainer.Close())
}

type requestGroup struct {
	mu      sync.Mutex
	wg      sync.WaitGroup
	stopped bool
}

func (g *requestGroup) wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.mu.Lock()
		if g.stopped {
			g.mu.Unlock()
			return
		}
		g.wg.Add(1)
		g.mu.Unlock()
		defer g.wg.Done()
		next.ServeHTTP(w, r)
	})
}

func (g *requestGroup) wait() {
	g.mu.Lock()
	g.stopped = true
	g.mu.Unlock()
	g.wg.Wait()
}
