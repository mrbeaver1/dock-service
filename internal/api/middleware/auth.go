package middleware

import (
	"context"
	"crypto/subtle"
	"errors"
	"log/slog"
	"net/http"
	"os"

	"github.com/google/uuid"
	"github.com/mrbeaver1/dock-service/internal/api/response"
	"github.com/mrbeaver1/dock-service/internal/models"
	"github.com/mrbeaver1/dock-service/internal/service"
)

type sessionKeyType string

const sessionKey sessionKeyType = "session"

type TokenSource func(*http.Request) (string, error)

type SessionValidator interface {
	Validate(context.Context, string) (models.Session, error)
}

func Auth(sessions SessionValidator, source TokenSource) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return withToken(source, func(w http.ResponseWriter, r *http.Request, tokenStr string) {
			session, err := sessions.Validate(r.Context(), tokenStr)
			if errors.Is(err, service.ErrInvalidSession) {
				response.Fail(w, r, http.StatusUnauthorized, http.StatusUnauthorized, "invalid token")
				return
			}
			if err != nil {
				slog.ErrorContext(r.Context(), "validate session", "error", err)
				response.Fail(w, r, 500, 500, "internal server error")
				return
			}
			ctx := context.WithValue(r.Context(), sessionKey, session)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func AdminAuth(expected string, source TokenSource) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return withToken(source, func(w http.ResponseWriter, r *http.Request, token string) {
			if expected == "" || subtle.ConstantTimeCompare([]byte(token), []byte(expected)) != 1 {
				response.Fail(w, r, 401, 401, "invalid administrator token")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func withToken(source TokenSource, next func(http.ResponseWriter, *http.Request, string)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if r.MultipartForm != nil {
				if err := r.MultipartForm.RemoveAll(); err != nil {
					slog.ErrorContext(r.Context(), "remove multipart files", "error", err)
				}
			}
		}()
		token, err := source(r)
		if err != nil {
			if WriteBodyError(w, r, err) {
				return
			}
			var pathErr *os.PathError
			if errors.As(err, &pathErr) {
				slog.ErrorContext(r.Context(), "read authentication form", "error", err)
				response.Fail(w, r, 500, 500, "internal server error")
			} else {
				response.Fail(w, r, 400, 400, "invalid authentication parameters")
			}
			return
		}
		if token == "" {
			response.Fail(w, r, 401, 401, "missing token")
			return
		}
		next(w, r, token)
	})
}

func UserIDFromContext(ctx context.Context) uuid.UUID {
	return SessionFromContext(ctx).UserID
}

func SessionFromContext(ctx context.Context) models.Session {
	return ctx.Value(sessionKey).(models.Session)
}
