package service

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type TokenIssuer interface {
	Issue(userID uuid.UUID) (string, error)
}

const sessionLifetime = time.Hour

type jwtIssuer struct{ secret []byte }

func NewJWTIssuer(secret []byte) (TokenIssuer, error) {
	if len(secret) == 0 {
		return nil, errors.New("JWT secret is required")
	}
	return &jwtIssuer{secret: bytes.Clone(secret)}, nil
}

func (s *jwtIssuer) Issue(userID uuid.UUID) (string, error) {
	if userID == uuid.Nil {
		return "", errors.New("token subject is required")
	}
	now := time.Now()
	id, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("generate session ID: %w", err)
	}
	claims := jwt.MapClaims{
		"jti": id.String(),
		"sub": userID.String(),
		"iat": now.Unix(),
		"exp": now.Add(sessionLifetime).Unix(),
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
	if err != nil {
		return "", fmt.Errorf("sign session token: %w", err)
	}
	return token, nil
}
