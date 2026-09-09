package middleware

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mrbeaver1/dock-service/internal/api/response"
)

var ErrBodyCapacity = errors.New("request body capacity exhausted")

type BodyLimiter struct {
	slots          chan struct{}
	mu             sync.Mutex
	used, capacity int64
	idle           time.Duration
	timeout        time.Duration
}

func NewBodyLimiter(concurrent int, capacity int64, idle, timeout time.Duration) (*BodyLimiter, error) {
	if concurrent < 1 || capacity < 1 || idle <= 0 || timeout <= 0 {
		return nil, errors.New("request body limits must be positive")
	}
	return &BodyLimiter{slots: make(chan struct{}, concurrent), capacity: capacity, idle: idle, timeout: timeout}, nil
}

func CloseRequestBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writer := &bodyResponseWriter{ResponseWriter: w, request: r}
		defer writer.closeBody()
		next.ServeHTTP(writer, r)
	})
}

type bodyResponseWriter struct {
	http.ResponseWriter
	request *http.Request
	closed  bool
}

func (w *bodyResponseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *bodyResponseWriter) closeBody() {
	if w.closed {
		return
	}
	w.closed = true
	body := w.request.Body
	if body == nil || body == http.NoBody {
		return
	}
	if body, ok := body.(*limitedBody); ok {
		_ = body.Close()
		return
	}
	controller := http.NewResponseController(w.ResponseWriter)
	_ = controller.SetReadDeadline(time.Now().Add(time.Second))
	_ = body.Close()
	_ = controller.SetReadDeadline(time.Time{})
}

func (w *bodyResponseWriter) WriteHeader(code int) {
	if code >= 200 || code == http.StatusSwitchingProtocols {
		w.closeBody()
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *bodyResponseWriter) Write(data []byte) (int, error) {
	w.closeBody()
	return w.ResponseWriter.Write(data)
}

func (w *bodyResponseWriter) FlushError() error {
	w.closeBody()
	return http.NewResponseController(w.ResponseWriter).Flush()
}

func (w *bodyResponseWriter) Flush() { _ = w.FlushError() }

func (l *BodyLimiter) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		controller := http.NewResponseController(w)
		select {
		case l.slots <- struct{}{}:
		default:
			_ = controller.SetReadDeadline(time.Now().Add(time.Second))
			w.Header().Set("Retry-After", "1")
			response.Fail(w, r, 503, 503, "request body capacity exhausted")
			return
		}
		body := &limitedBody{ReadCloser: r.Body, limiter: l, controller: controller, deadline: time.Now().Add(l.timeout)}
		r.Body = body
		defer func() {
			_ = body.Close()
			l.mu.Lock()
			l.used -= body.reserved
			l.mu.Unlock()
			<-l.slots
		}()
		next.ServeHTTP(w, r)
	})
}

type limitedBody struct {
	io.ReadCloser
	limiter    *BodyLimiter
	controller *http.ResponseController
	deadline   time.Time
	reserved   int64
	err        error
	closed     atomic.Bool
	closeOnce  sync.Once
	closeErr   error
	finished   bool
	finishErr  error
}

func (b *limitedBody) Read(p []byte) (int, error) {
	if b.closed.Load() {
		return 0, http.ErrBodyReadAfterClose
	}
	if len(p) == 0 {
		return 0, nil
	}
	if b.err != nil {
		return 0, b.err
	}
	now := time.Now()
	if !now.Before(b.deadline) {
		b.err = context.DeadlineExceeded
		return 0, b.err
	}
	if err := b.controller.SetReadDeadline(b.readDeadline(now.Add(b.limiter.idle))); err != nil && !errors.Is(err, http.ErrNotSupported) {
		return 0, err
	}
	l := b.limiter
	l.mu.Lock()
	allowed := min(int64(len(p)), l.capacity-l.used)
	l.used += allowed
	l.mu.Unlock()
	if allowed == 0 {
		var probe [1]byte
		n, err := b.ReadCloser.Read(probe[:])
		if n == 0 {
			return 0, err
		}
		b.err = ErrBodyCapacity
		return 0, b.err
	}
	n, err := b.ReadCloser.Read(p[:allowed])
	l.mu.Lock()
	l.used -= allowed - int64(n)
	l.mu.Unlock()
	b.reserved += int64(n)
	return n, err
}

func (b *limitedBody) Finish() error {
	if !b.finished {
		b.finished = true
		_, err := io.Copy(io.Discard, b)
		b.finishErr = errors.Join(err, b.Close())
	}
	return b.finishErr
}

func (b *limitedBody) Close() error {
	b.closeOnce.Do(func() {
		b.closed.Store(true)
		_ = b.controller.SetReadDeadline(b.readDeadline(time.Now().Add(min(b.limiter.idle, time.Second))))
		b.closeErr = b.ReadCloser.Close()
		_ = b.controller.SetReadDeadline(time.Time{})
	})
	return b.closeErr
}

func (b *limitedBody) readDeadline(deadline time.Time) time.Time {
	if b.deadline.Before(deadline) {
		return b.deadline
	}
	return deadline
}

func WriteBodyError(w http.ResponseWriter, r *http.Request, err error) bool {
	if errors.Is(err, ErrBodyCapacity) {
		w.Header().Set("Retry-After", "1")
		response.Fail(w, r, 503, 503, "request body capacity exhausted")
		return true
	}
	var networkErr net.Error
	if errors.As(err, &networkErr) && networkErr.Timeout() {
		response.Fail(w, r, 408, 408, "request body read timed out")
		return true
	}
	return false
}
