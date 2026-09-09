package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/mrbeaver1/dock-service/internal/api/response"
)

type userIDKeyType string

const userIDKey userIDKeyType = "user_id"

func Auth(secret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			tokenStr := strings.TrimPrefix(header, "Bearer ")
			// Если префикса не было, TrimPrefix вернёт исходную строку.
			if tokenStr == header || tokenStr == "" {
				response.Fail(w, r, http.StatusUnauthorized, http.StatusUnauthorized, "missing bearer token")
				return
			}

			token, err := jwt.Parse(tokenStr, func(_ *jwt.Token) (any, error) {
				return secret, nil
			})
			if err != nil || !token.Valid {
				response.Fail(w, r, http.StatusUnauthorized, http.StatusUnauthorized, "invalid token")
				return
			}

			claims, _ := token.Claims.(jwt.MapClaims)
			ctx := context.WithValue(r.Context(), userIDKey, claims["sub"])
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func UserIDFromContext(ctx context.Context) any {
	return ctx.Value(userIDKey)
}
