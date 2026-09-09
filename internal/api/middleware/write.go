package middleware

import (
	"errors"
	"io"
	"net/http"
	"time"
)

func WriteTimeout(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writer := &timeoutResponseWriter{ResponseWriter: w, timeout: timeout}
			next.ServeHTTP(writer, r)
			if err := writer.flush(); err != nil && !errors.Is(err, http.ErrNotSupported) {
				panic(http.ErrAbortHandler)
			}
		})
	}
}

type timeoutResponseWriter struct {
	http.ResponseWriter
	timeout time.Duration
	err     error
}

func (w *timeoutResponseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *timeoutResponseWriter) prepare() error {
	if w.err == nil {
		err := http.NewResponseController(w.ResponseWriter).SetWriteDeadline(time.Now().Add(w.timeout))
		if !errors.Is(err, http.ErrNotSupported) {
			w.err = err
		}
	}
	return w.err
}

func (w *timeoutResponseWriter) WriteHeader(code int) {
	if w.prepare() == nil {
		w.ResponseWriter.WriteHeader(code)
		w.clearDeadline()
	}
}

func (w *timeoutResponseWriter) clearDeadline() {
	err := http.NewResponseController(w.ResponseWriter).SetWriteDeadline(time.Time{})
	if w.err == nil && !errors.Is(err, http.ErrNotSupported) {
		w.err = err
	}
}

func (w *timeoutResponseWriter) Write(data []byte) (int, error) {
	written := 0
	for {
		if err := w.prepare(); err != nil {
			return written, err
		}
		chunk := data[:min(len(data), 32<<10)]
		n, err := w.ResponseWriter.Write(chunk)
		written += n
		if err == nil && n != len(chunk) {
			err = io.ErrShortWrite
		}
		if err != nil {
			w.err = err
			return written, err
		}
		w.clearDeadline()
		if w.err != nil {
			return written, w.err
		}
		data = data[n:]
		if len(data) == 0 {
			return written, nil
		}
	}
}

func (w *timeoutResponseWriter) FlushError() error {
	err := w.flush()
	if err == nil {
		w.clearDeadline()
	}
	return w.err
}

func (w *timeoutResponseWriter) flush() error {
	if err := w.prepare(); err != nil {
		return err
	}
	w.err = http.NewResponseController(w.ResponseWriter).Flush()
	return w.err
}

func (w *timeoutResponseWriter) Flush() { _ = w.FlushError() }
