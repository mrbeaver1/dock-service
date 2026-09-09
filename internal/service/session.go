package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/mrbeaver1/dock-service/internal/models"
)

var ErrInvalidSession = errors.New("invalid or expired session")

type SessionRepository interface {
	IsRevoked(context.Context, [32]byte) (bool, error)
	Revoke(context.Context, models.Session) error
}

type SessionService interface {
	Validate(context.Context, string) (models.Session, error)
	Revoke(context.Context, models.Session) error
}

type sessionService struct {
	repo   SessionRepository
	secret []byte
}

func NewSessionService(repo SessionRepository, secret []byte) (SessionService, error) {
	if len(secret) == 0 {
		return nil, errors.New("JWT secret is required")
	}
	return &sessionService{repo: repo, secret: bytes.Clone(secret)}, nil
}

func (s *sessionService) Validate(ctx context.Context, raw string) (models.Session, error) {
	claims := &jwt.RegisteredClaims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(_ *jwt.Token) (any, error) {
		return s.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil || !token.Valid {
		return models.Session{}, ErrInvalidSession
	}
	userID, err := uuid.Parse(claims.Subject)
	if err != nil || userID == uuid.Nil {
		return models.Session{}, ErrInvalidSession
	}
	session := models.Session{UserID: userID, TokenHash: sha256.Sum256(token.Signature)}
	if claims.ExpiresAt != nil {
		session.ExpiresAt = &claims.ExpiresAt.Time
	}
	revoked, err := s.repo.IsRevoked(ctx, session.TokenHash)
	if err != nil {
		return models.Session{}, fmt.Errorf("validate session: %w", err)
	}
	if revoked {
		return models.Session{}, ErrInvalidSession
	}
	return session, nil
}

func (s *sessionService) Revoke(ctx context.Context, session models.Session) error {
	return s.repo.Revoke(ctx, session)
}
