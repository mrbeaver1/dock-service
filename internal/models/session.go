package models

import (
	"time"

	"github.com/google/uuid"
)

type Session struct {
	UserID    uuid.UUID
	TokenHash [32]byte
	ExpiresAt *time.Time
}
