package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

func (s *documentService) DeleteDocument(ctx context.Context, id, userID uuid.UUID) error {
	document, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if document.OwnerID != userID {
		return ErrForbidden
	}
	collection, err := s.repo.GetCollection(ctx, document.OwnerID, nil)
	if err != nil {
		return err
	}
	deleted, err := s.repo.MarkDeleted(ctx, id, userID, document.Generation)
	if err != nil {
		return err
	}
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	defer func() {
		if err := s.cache.InvalidateDocumentLists(cleanup, collection); err != nil {
			slog.WarnContext(ctx, "invalidate deleted document lists", "owner_id", document.OwnerID, "error", err)
		}
		if err := s.cache.InvalidateDocument(cleanup, document); err != nil {
			slog.WarnContext(ctx, "invalidate deleted document", "document_id", id, "error", err)
		}
	}()
	if deleted.ObjectKey != nil {
		if err := s.s3.Delete(cleanup, *deleted.ObjectKey); err != nil {
			return err
		}
	}
	return s.repo.FinishDelete(cleanup, id, userID, deleted.Generation)
}
