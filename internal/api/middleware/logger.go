package middleware

import (
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) Unwrap() http.ResponseWriter { return rw.ResponseWriter }

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rw, r)

		slog.Info("http",
			"method", r.Method,
			"path", logPath(r),
			"status", rw.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", IDFromContext(r.Context()),
		)
	})
}

func logPath(r *http.Request) string {
	if r.Pattern == "" {
		return "<unmatched>"
	}
	if _, pattern, ok := strings.Cut(r.Pattern, " "); ok {
		return pattern
	}
	return r.Pattern
}
