package models

import "github.com/google/uuid"

type Deletion struct {
	ID           uuid.UUID
	OwnerID      uuid.UUID
	Generation   uuid.UUID
	ObjectKey    *string
	Version      int64
	CleanupToken uuid.UUID
}
