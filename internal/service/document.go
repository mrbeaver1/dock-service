package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"

	"github.com/google/uuid"
	"github.com/mrbeaver1/dock-service/internal/models"
)

var ErrForbidden = errors.New("document access denied")

type DocumentRepository interface {
	GetAccess(context.Context, uuid.UUID, uuid.UUID) (models.DocumentAccess, error)
	GetForRead(context.Context, uuid.UUID, uuid.UUID) (models.Document, bool, error)
}

type DocumentSource interface {
	Open(ctx context.Context, key string, offset int64) (io.ReadCloser, int64, error)
}

type DocumentCache interface {
	GetDocument(ctx context.Context, id uuid.UUID) (models.Document, error)
	SetDocument(ctx context.Context, document models.Document) error
	OpenFile(ctx context.Context, document models.Document) (io.ReadCloser, error)
	CacheFile(ctx context.Context, document models.Document, source io.ReadCloser) io.ReadCloser
}

type DocumentService interface {
	GetDocument(ctx context.Context, id, userID uuid.UUID, metadataOnly bool) (DocumentContent, error)
}

type DocumentContent struct {
	JSON json.RawMessage
	File *FileContent
}

type FileContent struct {
	MIME    string
	Size    int64
	Content io.ReadCloser
}

type documentService struct {
	repo  DocumentRepository
	s3    DocumentSource
	cache DocumentCache
}

func NewDocumentService(repo DocumentRepository, s3 DocumentSource, cache DocumentCache) DocumentService {
	return &documentService{repo: repo, s3: s3, cache: cache}
}

func (s *documentService) GetDocument(ctx context.Context, id, userID uuid.UUID, metadataOnly bool) (DocumentContent, error) {
	doc, err := s.loadDocument(ctx, id, userID)
	if err != nil {
		return DocumentContent{}, err
	}
	if doc.ObjectKey == nil {
		return DocumentContent{JSON: doc.JSON}, nil
	}
	if doc.FileSize == nil || *doc.FileSize < 0 {
		return DocumentContent{}, errors.New("invalid stored file size")
	}
	contentType := "application/octet-stream"
	if doc.MIME != nil && *doc.MIME != "" {
		kind, params, err := mime.ParseMediaType(*doc.MIME)
		if err != nil {
			return DocumentContent{}, fmt.Errorf("invalid stored MIME: %w", err)
		}
		contentType = mime.FormatMediaType(kind, params)
	}
	file := &FileContent{MIME: contentType, Size: *doc.FileSize}
	if metadataOnly {
		return DocumentContent{File: file}, nil
	}
	stream, err := s.cache.OpenFile(ctx, doc)
	if err == nil {
		file.Content = &cacheFallbackReader{ctx: ctx, source: stream, expected: file.Size,
			open: func(offset int64) (io.ReadCloser, error) { return s.openOrigin(ctx, doc, offset) }}
	} else {
		if ctx.Err() != nil {
			return DocumentContent{}, ctx.Err()
		}
		if !errors.Is(err, ErrCacheMiss) {
			slog.WarnContext(ctx, "open file cache", "document_id", id, "error", err)
		}
		stream, err = s.openOrigin(ctx, doc, 0)
		if err != nil {
			return DocumentContent{}, err
		}
		file.Content = s.cache.CacheFile(ctx, doc, stream)
	}
	return DocumentContent{File: file}, nil
}

func (s *documentService) openOrigin(ctx context.Context, doc models.Document, offset int64) (io.ReadCloser, error) {
	body, size, err := s.s3.Open(ctx, *doc.ObjectKey, offset)
	if err != nil {
		return nil, err
	}
	if size != *doc.FileSize-offset {
		return nil, errors.Join(fmt.Errorf("S3 object size differs from document metadata"), body.Close())
	}
	return body, nil
}

func (s *documentService) loadDocument(ctx context.Context, id, userID uuid.UUID) (models.Document, error) {
	doc, err := s.cache.GetDocument(ctx, id)
	if err == nil {
		access, err := s.repo.GetAccess(ctx, id, userID)
		if err != nil {
			return models.Document{}, err
		}
		if !access.Allowed {
			return models.Document{}, ErrForbidden
		}
		if doc.Generation == access.Generation && doc.Version == access.Version {
			return doc, nil
		}
	} else {
		if ctx.Err() != nil {
			return models.Document{}, ctx.Err()
		}
		if !errors.Is(err, ErrCacheMiss) {
			slog.WarnContext(ctx, "read document cache", "document_id", id, "error", err)
		}
	}
	doc, allowed, err := s.repo.GetForRead(ctx, id, userID)
	if err != nil {
		return models.Document{}, err
	}
	if !allowed {
		return models.Document{}, ErrForbidden
	}
	if err := s.cache.SetDocument(ctx, doc); err != nil {
		slog.WarnContext(ctx, "write document cache", "document_id", id, "error", err)
	}
	return doc, nil
}

type cacheFallbackReader struct {
	ctx              context.Context
	source           io.ReadCloser
	expected, offset int64
	open             func(int64) (io.ReadCloser, error)
	fallback         bool
}

func (r *cacheFallbackReader) Read(p []byte) (int, error) {
	n, err := r.source.Read(p)
	r.offset += int64(n)
	if err == nil || (err == io.EOF && r.offset == r.expected) || r.fallback || r.ctx.Err() != nil {
		return n, err
	}
	_ = r.source.Close()
	source, openErr := r.open(r.offset)
	if openErr != nil {
		return n, openErr
	}
	slog.WarnContext(r.ctx, "resume expired/unavailable file cache from S3", "offset", r.offset, "error", err)
	r.source = source
	r.fallback = true
	if n > 0 {
		return n, nil
	}
	return r.Read(p)
}

func (r *cacheFallbackReader) Close() error { return r.source.Close() }
