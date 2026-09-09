package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/mrbeaver1/dock-service/internal/models"
	goredis "github.com/redis/go-redis/v9"
)

var ErrCacheMiss = errors.New("cache miss")

type Cache interface {
	GetDocument(ctx context.Context, id uuid.UUID) (models.Document, error)
	SetDocument(ctx context.Context, document models.Document) error
	OpenFile(ctx context.Context, document models.Document) (io.ReadCloser, error)
	CacheFile(ctx context.Context, document models.Document, source io.ReadCloser) io.ReadCloser
	InvalidateDocument(ctx context.Context, document models.Document) error
}

type redisCache struct {
	client *goredis.Client
	ttl    time.Duration
}

func NewCache(client *goredis.Client, ttl time.Duration) (Cache, error) {
	if ttl < time.Millisecond {
		return nil, fmt.Errorf("cache TTL must be at least one millisecond")
	}
	return &redisCache{client: client, ttl: ttl}, nil
}

func cachePrefix(id uuid.UUID) string      { return "dock:v2:{" + id.String() + "}:" }
func documentCacheKey(id uuid.UUID) string { return cachePrefix(id) + "document" }
func fileCacheKey(doc models.Document) string {
	return cachePrefix(doc.ID) + "file:" + doc.Generation.String() + ":" + strconv.FormatInt(doc.Version, 10)
}

func (c *redisCache) GetDocument(ctx context.Context, id uuid.UUID) (models.Document, error) {
	data, err := c.client.Get(ctx, documentCacheKey(id)).Bytes()
	if errors.Is(err, goredis.Nil) {
		return models.Document{}, ErrCacheMiss
	}
	if err != nil {
		return models.Document{}, fmt.Errorf("get cached document: %w", err)
	}
	var doc models.Document
	if err := json.Unmarshal(data, &doc); err != nil {
		return doc, fmt.Errorf("decode cached document: %w", err)
	}
	if doc.ID != id || doc.Generation == uuid.Nil || doc.Version < 1 {
		return doc, fmt.Errorf("invalid cached document identity/version")
	}
	return doc, nil
}

func (c *redisCache) SetDocument(ctx context.Context, doc models.Document) error {
	data, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("encode cached document: %w", err)
	}
	if err := c.client.Set(ctx, documentCacheKey(doc.ID), data, c.ttl).Err(); err != nil {
		return fmt.Errorf("set cached document: %w", err)
	}
	return nil
}

func (c *redisCache) InvalidateDocument(ctx context.Context, doc models.Document) error {
	if err := c.client.Unlink(ctx, documentCacheKey(doc.ID), fileCacheKey(doc)).Err(); err != nil {
		return fmt.Errorf("invalidate document cache: %w", err)
	}
	return nil
}
