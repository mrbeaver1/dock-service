package dto

import (
	"encoding/json"
	"io"

	"github.com/google/uuid"
	"github.com/mrbeaver1/dock-service/internal/models"
)

type CreateDocumentRequest struct {
	Meta *DocumentMeta
	JSON json.RawMessage
	File *UploadedFile
}

type DocumentMeta struct {
	Name   *string  `json:"name"`
	File   *bool    `json:"file"`
	Public *bool    `json:"public"`
	MIME   *string  `json:"mime"`
	Grant  []string `json:"grant"`
}

type UploadedFile struct {
	Filename string
	Size     int64
	Content  io.ReadSeeker
}

type CreateDocumentResult struct {
	JSON json.RawMessage `json:"json,omitempty"`
	File *string         `json:"file,omitempty"`
}

type GetDocumentsRequest = models.DocumentListOptions

type GetDocumentsResult struct {
	Docs []DocumentListItem `json:"docs"`
}

type DocumentListItem struct {
	ID      uuid.UUID `json:"id"`
	Name    *string   `json:"name"`
	MIME    *string   `json:"mime"`
	File    *bool     `json:"file"`
	Public  *bool     `json:"public"`
	Created string    `json:"created"`
	Grant   []string  `json:"grant"`
}
