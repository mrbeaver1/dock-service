package models

import (
	"time"

	"github.com/google/uuid"
)

type Grant struct {
	ID         uuid.UUID
	DocumentID uuid.UUID
	UserID     uuid.UUID
	CreatedAt  time.Time
}
