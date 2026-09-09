package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/mrbeaver1/dock-service/internal/models"
	"golang.org/x/net/http/httpguts"
)

var ErrInvalidDocument = errors.New("invalid document")

func (s *documentService) UploadDocument(ctx context.Context, document models.Document, grants []string, content io.Reader) (models.Document, error) {
	if err := prepareDocument(&document, content); err != nil {
		return models.Document{}, err
	}
	collection, err := s.repo.GetCollection(ctx, document.OwnerID, nil)
	if errors.Is(err, models.ErrUserNotFound) {
		return models.Document{}, ErrForbidden
	}
	if err != nil {
		return models.Document{}, err
	}
	userIDs, err := s.resolveGrants(ctx, grants)
	if err != nil {
		return models.Document{}, err
	}
	document.ID, err = uuid.NewV7()
	if err != nil {
		return models.Document{}, fmt.Errorf("generate document ID: %w", err)
	}
	document.ObjectKey = nil
	if content != nil {
		operation, cancel := context.WithTimeout(ctx, s.uploadTimeout)
		defer cancel()
		ctx = operation
		key := "documents/" + document.ID.String()
		document.ObjectKey = &key
		if err := s.uploads.Create(ctx, models.Upload{ID: document.ID, OwnerID: document.OwnerID, ObjectKey: key}, s.uploadTimeout); err != nil {
			return models.Document{}, err
		}
		defer s.scheduleUploadCleanup(ctx, document.ID)
		if err := s.s3.Put(ctx, key, content, *document.FileSize, "application/octet-stream"); err != nil {
			s.removeUploadedObject(ctx, key)
			return models.Document{}, err
		}
	} else {
		document.FileSize = nil
	}
	saved, err := s.repo.Create(ctx, document, userIDs)
	if err != nil && (document.ObjectKey != nil || errors.Is(err, models.ErrCommitUnknown)) {
		saved, err = s.resolveUploadFailure(ctx, document, err)
	}
	if err != nil {
		return models.Document{}, err
	}
	if err := s.cache.InvalidateDocumentLists(ctx, collection); err != nil {
		slog.WarnContext(ctx, "invalidate document lists", "owner_id", collection.OwnerID, "error", err)
	}
	if err := s.cache.InvalidateDocument(ctx, saved); err != nil {
		slog.WarnContext(ctx, "invalidate uploaded document", "document_id", saved.ID, "error", err)
	}
	return saved, nil
}

func (s *documentService) scheduleUploadCleanup(ctx context.Context, id uuid.UUID) {
	check, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := s.uploads.Schedule(check, id); err != nil {
		slog.ErrorContext(ctx, "schedule upload recovery", "document_id", id, "error", err)
	}
}

func prepareDocument(document *models.Document, content io.Reader) error {
	if document.Name != nil && !validStoredText(*document.Name) {
		return fmt.Errorf("%w: name must be UTF-8 without NUL", ErrInvalidDocument)
	}
	if document.MIME != nil && *document.MIME != "" {
		value := *document.MIME
		kind, params, err := mime.ParseMediaType(value)
		if err != nil || !strings.Contains(kind, "/") || !utf8.ValidString(value) || !httpguts.ValidHeaderFieldValue(value) {
			return fmt.Errorf("%w: mime must be a valid media type", ErrInvalidDocument)
		}
		for _, value := range params {
			if !utf8.ValidString(value) || !httpguts.ValidHeaderFieldValue(value) {
				return fmt.Errorf("%w: mime must contain valid media parameters", ErrInvalidDocument)
			}
		}
		contentType := mime.FormatMediaType(kind, params)
		document.MIME = &contentType
	}
	if document.JSON != nil && !json.Valid(document.JSON) {
		return fmt.Errorf("%w: json must contain valid JSON", ErrInvalidDocument)
	}
	if content != nil && (document.FileSize == nil || *document.FileSize < 0) {
		return fmt.Errorf("%w: file size is required", ErrInvalidDocument)
	}
	return nil
}

func validStoredText(value string) bool {
	return utf8.ValidString(value) && !strings.ContainsRune(value, '\x00')
}

func (s *documentService) resolveGrants(ctx context.Context, logins []string) ([]uuid.UUID, error) {
	if len(logins) == 0 {
		return nil, nil
	}
	logins = slices.Clone(logins)
	slices.Sort(logins)
	logins = slices.Compact(logins)
	for _, login := range logins {
		if !validStoredText(login) {
			return nil, fmt.Errorf("%w: grant must contain UTF-8 logins without NUL", ErrInvalidDocument)
		}
	}
	users, err := s.users.GetIDsByLogins(ctx, logins)
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, 0, len(logins))
	for _, login := range logins {
		id, ok := users[login]
		if !ok {
			return nil, fmt.Errorf("%w: grant contains an unknown login", ErrInvalidDocument)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (s *documentService) removeUploadedObject(ctx context.Context, key string) {
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if err := s.s3.Delete(cleanup, key); err != nil {
		slog.ErrorContext(ctx, "remove failed upload", "object_key", key, "error", err)
	}
}

func (s *documentService) resolveUploadFailure(ctx context.Context, document models.Document, createErr error) (models.Document, error) {
	if !errors.Is(createErr, models.ErrCommitUnknown) {
		if document.ObjectKey != nil {
			s.removeUploadedObject(ctx, *document.ObjectKey)
		}
		return models.Document{}, createErr
	}
	check, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	saved, err := s.repo.GetByID(check, document.ID)
	sameObject := saved.ObjectKey == nil && document.ObjectKey == nil
	if saved.ObjectKey != nil && document.ObjectKey != nil {
		sameObject = *saved.ObjectKey == *document.ObjectKey
	}
	if err == nil && saved.ID == document.ID && saved.OwnerID == document.OwnerID && sameObject {
		return saved, nil
	}
	slog.ErrorContext(ctx, "upload persistence could not be confirmed", "document_id", document.ID,
		"create_error", createErr, "read_error", err)
	return models.Document{}, createErr
}
