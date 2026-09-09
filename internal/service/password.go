package service

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

type PasswordHasher interface {
	Hash(context.Context, string) (string, error)
	Verify(context.Context, string, string) (bool, error)
}

var ErrBusy = errors.New("service capacity exhausted")

const (
	passwordMemory      = 19 * 1024
	passwordIterations  = 2
	passwordParallelism = 1
	passwordSaltSize    = 16
	passwordKeySize     = 32
)

var passwordHashPrefix = fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$",
	argon2.Version, passwordMemory, passwordIterations, passwordParallelism)

type passwordHasher struct {
	slots     chan struct{}
	admission chan struct{}
}

func NewPasswordHasher(concurrent int) (PasswordHasher, error) {
	if concurrent < 1 || concurrent > int(^uint(0)>>1)/8 {
		return nil, errors.New("password concurrency must be positive")
	}
	return &passwordHasher{slots: make(chan struct{}, concurrent), admission: make(chan struct{}, concurrent*8)}, nil
}

func (h *passwordHasher) acquire(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case h.admission <- struct{}{}:
	default:
		return ErrBusy
	}
	select {
	case h.slots <- struct{}{}:
		if err := ctx.Err(); err != nil {
			h.release()
			return err
		}
		return nil
	case <-ctx.Done():
		<-h.admission
		return ctx.Err()
	}
}

func (h *passwordHasher) release() { <-h.slots; <-h.admission }

func (h *passwordHasher) Hash(ctx context.Context, password string) (string, error) {
	if err := h.acquire(ctx); err != nil {
		return "", err
	}
	defer h.release()
	salt := make([]byte, passwordSaltSize)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, passwordIterations, passwordMemory, passwordParallelism, passwordKeySize)
	return passwordHashPrefix + base64.RawStdEncoding.EncodeToString(salt) + "$" + base64.RawStdEncoding.EncodeToString(key), nil
}

func (h *passwordHasher) Verify(ctx context.Context, encoded, password string) (bool, error) {
	if err := h.acquire(ctx); err != nil {
		return false, err
	}
	defer h.release()
	data, ok := strings.CutPrefix(encoded, passwordHashPrefix)
	if !ok {
		return false, errors.New("unsupported password hash format")
	}
	saltText, keyText, ok := strings.Cut(data, "$")
	if !ok || len(saltText) != base64.RawStdEncoding.EncodedLen(passwordSaltSize) || len(keyText) != base64.RawStdEncoding.EncodedLen(passwordKeySize) {
		return false, errors.New("invalid password hash format")
	}
	salt, saltErr := base64.RawStdEncoding.Strict().DecodeString(saltText)
	want, keyErr := base64.RawStdEncoding.Strict().DecodeString(keyText)
	if saltErr != nil || keyErr != nil || len(salt) != passwordSaltSize || len(want) != passwordKeySize {
		return false, errors.New("invalid password hash encoding")
	}
	got := argon2.IDKey([]byte(password), salt, passwordIterations, passwordMemory, passwordParallelism, passwordKeySize)
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}
