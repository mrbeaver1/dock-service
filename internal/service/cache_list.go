package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/mrbeaver1/dock-service/internal/models"
	goredis "github.com/redis/go-redis/v9"
)

var cacheListScript = goredis.NewScript(`
redis.call('HSET', KEYS[1], ARGV[1], ARGV[2])
redis.call('HPEXPIRE', KEYS[1], ARGV[3], 'FIELDS', 1, ARGV[1])
redis.call('PEXPIRE', KEYS[1], ARGV[3])
return 1
`)

func documentListKey(collection models.DocumentCollection) string {
	return "dock:v3:{" + collection.OwnerID.String() + "}:lists:" + collection.Revision.String()
}

func documentListField(query models.DocumentListQuery) (string, error) {
	data, err := json.Marshal(query)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(data)), nil
}

func (c *redisCache) GetDocumentList(ctx context.Context, collection models.DocumentCollection, query models.DocumentListQuery) ([]models.DocumentListItem, error) {
	field, err := documentListField(query)
	if err != nil {
		return nil, err
	}
	data, err := c.client.HGet(ctx, documentListKey(collection), field).Bytes()
	if errors.Is(err, goredis.Nil) {
		return nil, ErrCacheMiss
	}
	if err != nil {
		return nil, fmt.Errorf("get cached document list: %w", err)
	}
	var documents []models.DocumentListItem
	if err := json.Unmarshal(data, &documents); err != nil {
		return nil, fmt.Errorf("decode cached document list: %w", err)
	}
	if documents == nil {
		return nil, errors.New("invalid cached document list")
	}
	return documents, nil
}

func (c *redisCache) SetDocumentList(ctx context.Context, collection models.DocumentCollection, query models.DocumentListQuery, documents []models.DocumentListItem) error {
	field, err := documentListField(query)
	if err != nil {
		return err
	}
	data, err := json.Marshal(documents)
	if err != nil {
		return fmt.Errorf("encode cached document list: %w", err)
	}
	if err := cacheListScript.Run(ctx, c.client, []string{documentListKey(collection)}, field, data, c.ttl.Milliseconds()).Err(); err != nil {
		return fmt.Errorf("set cached document list: %w", err)
	}
	return nil
}

func (c *redisCache) InvalidateDocumentLists(ctx context.Context, collection models.DocumentCollection) error {
	if err := c.client.Unlink(ctx, documentListKey(collection)).Err(); err != nil {
		return fmt.Errorf("invalidate document lists: %w", err)
	}
	return nil
}
