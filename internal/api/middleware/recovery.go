package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/mrbeaver1/dock-service/internal/api/response"
)

func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				if rec == http.ErrAbortHandler {
					panic(rec)
				}
				slog.Error("panic recovered",
					"panic", rec,
					"method", r.Method,
					"path", logPath(r),
					"stack", string(debug.Stack()),
				)
				response.Fail(w, r, http.StatusInternalServerError, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
