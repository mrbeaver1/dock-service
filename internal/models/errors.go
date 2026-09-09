package models

import "errors"

var (
	ErrUploadExpired    = errors.New("upload is no longer active")
	ErrDocumentNotFound = errors.New("document not found")
	ErrUserNotFound     = errors.New("user not found")
	ErrConflict         = errors.New("record already exists")
	ErrInvalidData      = errors.New("invalid document data")
	ErrCommitUnknown    = errors.New("transaction outcome is unknown")
)
