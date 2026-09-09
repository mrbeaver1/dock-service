package models

import "github.com/google/uuid"

type Upload struct {
	ID           uuid.UUID
	OwnerID      uuid.UUID
	ObjectKey    string
	CleanupToken uuid.UUID
}
