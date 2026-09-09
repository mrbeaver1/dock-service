package models

import (
	"time"

	"github.com/google/uuid"
)

type DocumentListOptions struct {
	Login *string
	Key   *string
	Value *string
	Limit *uint64
}

type DocumentCollection struct {
	OwnerID  uuid.UUID
	Revision uuid.UUID
}

type DocumentFilter struct {
	Field string
	Text  string
	ID    uuid.UUID
	Bool  bool
	From  time.Time
	Until time.Time
}

type DocumentListQuery struct {
	OwnerID  uuid.UUID
	ViewerID uuid.UUID
	Filter   *DocumentFilter
	Limit    *uint64
}

type DocumentListItem struct {
	ID        uuid.UUID
	Name      *string
	MIME      *string
	File      *bool
	Public    *bool
	CreatedAt time.Time
	Grant     []string
}
