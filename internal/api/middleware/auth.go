package middleware

import (
	"context"
	"crypto/subtle"
	"errors"
	"log/slog"
	"net/http"
	"os"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/mrbeaver1/dock-service/internal/api/response"
)

type userIDKeyType string

const userIDKey userIDKeyType = "user_id"

type TokenSource func(*http.Request) (string, error)

func Auth(secret []byte, source TokenSource) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return withToken(source, func(w http.ResponseWriter, r *http.Request, tokenStr string) {
			claims := &jwt.RegisteredClaims{}
			token, err := jwt.ParseWithClaims(tokenStr, claims, func(_ *jwt.Token) (any, error) {
				return secret, nil
			}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
			if err != nil || !token.Valid {
				response.Fail(w, r, http.StatusUnauthorized, http.StatusUnauthorized, "invalid token")
				return
			}

			userID, err := uuid.Parse(claims.Subject)
			if err != nil || userID == uuid.Nil {
				response.Fail(w, r, 401, 401, "invalid token subject")
				return
			}
			ctx := context.WithValue(r.Context(), userIDKey, userID)
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
	return ctx.Value(userIDKey).(uuid.UUID)
}
