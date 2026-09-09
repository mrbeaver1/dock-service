package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/mrbeaver1/dock-service/internal/models"
)

var ErrInvalidListOptions = errors.New("invalid list parameters")

func (s *documentService) GetDocumentsList(ctx context.Context, viewerID uuid.UUID, options models.DocumentListOptions) ([]models.DocumentListItem, error) {
	if options.Login != nil && !validStoredText(*options.Login) {
		return nil, fmt.Errorf("%w: login must be UTF-8 without NUL", ErrInvalidListOptions)
	}
	filter, err := documentFilter(options.Key, options.Value)
	if err != nil {
		return nil, err
	}
	collection, err := s.repo.GetCollection(ctx, viewerID, options.Login)
	if err != nil {
		return nil, err
	}
	query := models.DocumentListQuery{OwnerID: collection.OwnerID, ViewerID: viewerID, Filter: filter, Limit: options.Limit}
	if query.Limit != nil && *query.Limit > uint64(1<<63-1) {
		query.Limit = nil
	}
	documents, err := s.cache.GetDocumentList(ctx, collection, query)
	if err == nil {
		return documents, nil
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if !errors.Is(err, ErrCacheMiss) {
		slog.WarnContext(ctx, "read document list cache", "owner_id", collection.OwnerID, "error", err)
	}
	documents, err = s.repo.List(ctx, query)
	if err != nil {
		return nil, err
	}
	if err := s.cache.SetDocumentList(ctx, collection, query, documents); err != nil {
		slog.WarnContext(ctx, "write document list cache", "owner_id", collection.OwnerID, "error", err)
	}
	return documents, nil
}

func documentFilter(key, value *string) (*models.DocumentFilter, error) {
	if key == nil && value == nil {
		return nil, nil
	}
	if key == nil || value == nil {
		return nil, fmt.Errorf("%w: key and value must be provided together", ErrInvalidListOptions)
	}
	filter := &models.DocumentFilter{Field: *key}
	switch filter.Field {
	case "name", "mime", "grant":
		if !validStoredText(*value) {
			return nil, fmt.Errorf("%w: value must be UTF-8 without NUL", ErrInvalidListOptions)
		}
		filter.Text = *value
	case "id":
		id, err := uuid.Parse(*value)
		if err != nil {
			return nil, fmt.Errorf("%w: id must be a UUID", ErrInvalidListOptions)
		}
		filter.ID = id
	case "file", "public":
		flag, err := strconv.ParseBool(*value)
		if err != nil {
			return nil, fmt.Errorf("%w: value must be a boolean", ErrInvalidListOptions)
		}
		filter.Bool = flag
	case "created", "created_at":
		created, err := time.Parse(time.DateTime, *value)
		if err != nil {
			created, err = time.Parse(time.RFC3339Nano, *value)
		}
		if err != nil {
			return nil, fmt.Errorf("%w: created must be RFC3339 or YYYY-MM-DD HH:MM:SS", ErrInvalidListOptions)
		}
		filter.Field = "created"
		filter.From = created.UTC().Truncate(time.Second)
		filter.Until = filter.From.Add(time.Second)
	default:
		return nil, fmt.Errorf("%w: unsupported key", ErrInvalidListOptions)
	}
	return filter, nil
}
