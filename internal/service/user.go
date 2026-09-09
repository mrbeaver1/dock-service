package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/mrbeaver1/dock-service/internal/models"
)

const (
	minLoginLength = 8
	maxLoginLength = 256
)

var (
	ErrInvalidLogin       = fmt.Errorf("login must contain %d to %d ASCII letters or digits", minLoginLength, maxLoginLength)
	ErrInvalidPassword    = errors.New("password must contain at least 8 characters, uppercase and lowercase letters, a digit and a special character")
	ErrLoginTaken         = errors.New("login is already registered")
	ErrInvalidCredentials = errors.New("invalid login or password")
)

type UserRepository interface {
	GetByID(context.Context, uuid.UUID) (models.User, error)
	GetByLogin(context.Context, string) (models.User, error)
	Create(context.Context, models.User) (models.User, error)
}

type UserService interface {
	GetByID(context.Context, uuid.UUID) (models.User, error)
	GetByLogin(context.Context, string) (models.User, error)
	Register(ctx context.Context, login, password string) (string, error)
	Authenticate(ctx context.Context, login, password string) (string, error)
}

type userService struct {
	repo      UserRepository
	passwords PasswordHasher
	tokens    TokenIssuer
}

func NewUserService(repo UserRepository, passwords PasswordHasher, tokens TokenIssuer) UserService {
	return &userService{
		repo: repo, passwords: passwords, tokens: tokens,
	}
}

func (s *userService) GetByID(ctx context.Context, id uuid.UUID) (models.User, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *userService) GetByLogin(ctx context.Context, login string) (models.User, error) {
	return s.repo.GetByLogin(ctx, login)
}

func (s *userService) Register(ctx context.Context, login, password string) (string, error) {
	if !validLogin(login) {
		return "", ErrInvalidLogin
	}
	if !validPassword(password) {
		return "", ErrInvalidPassword
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	hash, err := s.passwords.Hash(password)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	user, err := s.repo.Create(ctx, models.User{Login: login, PasswordHash: hash})
	if errors.Is(err, models.ErrConflict) {
		return "", ErrLoginTaken
	}
	if err != nil {
		return "", fmt.Errorf("register user: %w", err)
	}
	return user.Login, nil
}

func (s *userService) Authenticate(ctx context.Context, login, password string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if !utf8.ValidString(login) || strings.ContainsRune(login, '\x00') {
		return "", ErrInvalidCredentials
	}
	user, err := s.repo.GetByLogin(ctx, login)
	if errors.Is(err, models.ErrUserNotFound) {
		if _, err := s.passwords.Hash(password); err != nil {
			return "", fmt.Errorf("hash authentication candidate: %w", err)
		}
		return "", ErrInvalidCredentials
	}
	if err != nil {
		return "", fmt.Errorf("find authenticating user: %w", err)
	}
	match, err := s.passwords.Verify(user.PasswordHash, password)
	if err != nil {
		return "", fmt.Errorf("verify password: %w", err)
	}
	if !match {
		return "", ErrInvalidCredentials
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return s.tokens.Issue(user.ID)
}

func validLogin(login string) bool {
	if len(login) < minLoginLength || len(login) > maxLoginLength {
		return false
	}
	for _, r := range login {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}

func validPassword(password string) bool {
	if !utf8.ValidString(password) || utf8.RuneCountInString(password) < 8 {
		return false
	}
	var upper, lower, digit, special bool
	for _, r := range password {
		upper = upper || unicode.IsUpper(r)
		lower = lower || unicode.IsLower(r)
		digit = digit || unicode.IsDigit(r)
		special = special || (!unicode.IsLetter(r) && !unicode.IsDigit(r))
	}
	return upper && lower && digit && special
}
