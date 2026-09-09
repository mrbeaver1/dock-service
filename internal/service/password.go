package service

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

type PasswordHasher interface {
	Hash(password string) (string, error)
	Verify(encoded, password string) (bool, error)
}

const (
	passwordMemory      = 19 * 1024
	passwordIterations  = 2
	passwordParallelism = 1
	passwordSaltSize    = 16
	passwordKeySize     = 32
)

var passwordHashPrefix = fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$",
	argon2.Version, passwordMemory, passwordIterations, passwordParallelism)

type passwordHasher struct{}

func NewPasswordHasher() PasswordHasher { return &passwordHasher{} }

func (h *passwordHasher) Hash(password string) (string, error) {
	salt := make([]byte, passwordSaltSize)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, passwordIterations, passwordMemory, passwordParallelism, passwordKeySize)
	return passwordHashPrefix + base64.RawStdEncoding.EncodeToString(salt) + "$" + base64.RawStdEncoding.EncodeToString(key), nil
}

func (h *passwordHasher) Verify(encoded, password string) (bool, error) {
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
