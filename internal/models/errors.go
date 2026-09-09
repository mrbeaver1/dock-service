package models

import "errors"

var (
	ErrDocumentNotFound = errors.New("document not found")
	ErrUserNotFound     = errors.New("user not found")
	ErrConflict         = errors.New("record already exists")
)
