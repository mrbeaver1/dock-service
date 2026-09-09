package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Document struct {
	ID         uuid.UUID       `json:"id"`
	Generation uuid.UUID       `json:"generation"`
	OwnerID    uuid.UUID       `json:"owner_id"`
	Name       *string         `json:"name"`
	File       *bool           `json:"file"`
	Public     *bool           `json:"public"`
	MIME       *string         `json:"mime"`
	JSON       json.RawMessage `json:"json,omitempty"`
	ObjectKey  *string         `json:"object_key"`
	FileSize   *int64          `json:"file_size"`
	Version    int64           `json:"version"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
	DeletedAt  *time.Time      `json:"deleted_at,omitempty"`
}

type DocumentAccess struct {
	Generation uuid.UUID
	Version    int64
	Allowed    bool
}
