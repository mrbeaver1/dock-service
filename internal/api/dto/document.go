package dto

import (
	"encoding/json"
	"io"
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

type GetDocumentsRequest struct {
	Login *string
	Key   *string
	Value *string
	Limit *uint64
}

type GetDocumentsResult struct {
	Docs []json.RawMessage `json:"docs,omitempty"`
}
