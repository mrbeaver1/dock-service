package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strconv"

	"github.com/google/uuid"
	"github.com/mrbeaver1/dock-service/internal/models"
	goredis "github.com/redis/go-redis/v9"
)

const fileCacheChunkSize = 1 << 20

var cacheChunkScript = goredis.NewScript(`
redis.call('HSET', KEYS[1], ARGV[1], ARGV[2])
redis.call('PEXPIRE', KEYS[1], ARGV[3])
return 1
`)

var publishFileScript = goredis.NewScript(`
if redis.call('HLEN', KEYS[1]) ~= tonumber(ARGV[1]) then
    redis.call('UNLINK', KEYS[1])
    return 0
end
redis.call('HSET', KEYS[1], 'chunks', ARGV[1], 'size', ARGV[2])
redis.call('PEXPIRE', KEYS[1], ARGV[3])
redis.call('UNLINK', KEYS[2])
redis.call('RENAME', KEYS[1], KEYS[2])
return 1
`)

func (c *redisCache) OpenFile(ctx context.Context, doc models.Document) (io.ReadCloser, error) {
	if doc.FileSize == nil {
		return nil, ErrCacheMiss
	}
	key := fileCacheKey(doc)
	values, err := c.client.HMGet(ctx, key, "size", "chunks").Result()
	if err != nil {
		return nil, fmt.Errorf("get cached file manifest: %w", err)
	}
	if values[0] == nil || values[1] == nil {
		return nil, ErrCacheMiss
	}
	sizeText, sizeOK := values[0].(string)
	chunksText, chunksOK := values[1].(string)
	size, sizeErr := strconv.ParseInt(sizeText, 10, 64)
	chunks, chunksErr := strconv.ParseInt(chunksText, 10, 64)
	if !sizeOK || !chunksOK || sizeErr != nil || chunksErr != nil || size < 0 || size != *doc.FileSize || chunks != chunkCount(size) {
		return nil, fmt.Errorf("invalid cached file manifest")
	}
	r := &redisFileReader{ctx: ctx, client: c.client, key: key, size: size}
	if size > 0 {
		if err := r.next(); err != nil {
			return nil, err
		}
	}
	return r, nil
}

func chunkCount(size int64) int64 {
	count := size / fileCacheChunkSize
	if size%fileCacheChunkSize != 0 {
		count++
	}
	return count
}

type redisFileReader struct {
	ctx                 context.Context
	client              *goredis.Client
	key                 string
	size, offset, chunk int64
	buffer              []byte
	closed              bool
}

func (r *redisFileReader) next() error {
	data, err := r.client.HGet(r.ctx, r.key, "chunk:"+strconv.FormatInt(r.chunk, 10)).Bytes()
	if errors.Is(err, goredis.Nil) {
		return ErrCacheMiss
	}
	if err != nil {
		return fmt.Errorf("read cached file block: %w", err)
	}
	expected := min(int64(fileCacheChunkSize), r.size-r.offset)
	if int64(len(data)) != expected {
		return fmt.Errorf("invalid cached file block size")
	}
	r.buffer = data
	r.chunk++
	return nil
}

func (r *redisFileReader) Read(p []byte) (int, error) {
	if r.closed {
		return 0, io.ErrClosedPipe
	}
	if len(p) == 0 {
		return 0, nil
	}
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	if r.offset == r.size {
		return 0, io.EOF
	}
	if len(r.buffer) == 0 {
		if err := r.next(); err != nil {
			return 0, err
		}
	}
	n := copy(p, r.buffer)
	r.buffer = r.buffer[n:]
	r.offset += int64(n)
	return n, nil
}

func (r *redisFileReader) Close() error { r.closed = true; r.buffer = nil; return nil }

func (c *redisCache) CacheFile(ctx context.Context, doc models.Document, source io.ReadCloser) io.ReadCloser {
	if doc.FileSize == nil {
		return source
	}
	id, err := uuid.NewV7()
	if err != nil {
		slog.WarnContext(ctx, "create cache fill ID", "error", err)
		return source
	}
	key := fileCacheKey(doc)
	return &redisFileFill{ctx: ctx, cache: c, source: source, key: key,
		stage: key + ":fill:" + id.String(), expected: *doc.FileSize,
		buffer: make([]byte, 0, fileCacheChunkSize)}
}

type redisFileFill struct {
	ctx                    context.Context
	cache                  *redisCache
	source                 io.ReadCloser
	key, stage             string
	expected, read, chunks int64
	buffer                 []byte
	disabled, published    bool
}

func (r *redisFileFill) discard(err error) {
	if r.disabled || r.published {
		return
	}
	r.disabled = true
	r.buffer = nil
	if err != nil {
		slog.WarnContext(r.ctx, "fill file cache", "error", err)
	}
	if err := r.cache.client.Unlink(r.ctx, r.stage).Err(); err != nil && r.ctx.Err() == nil {
		slog.WarnContext(r.ctx, "discard file cache fill", "error", err)
	}
}

func (r *redisFileFill) flush() error {
	if len(r.buffer) == 0 {
		return nil
	}
	err := cacheChunkScript.Run(r.ctx, r.cache.client, []string{r.stage},
		"chunk:"+strconv.FormatInt(r.chunks, 10), r.buffer, r.cache.ttl.Milliseconds()).Err()
	if err != nil {
		return err
	}
	r.chunks++
	r.buffer = r.buffer[:0]
	return nil
}

func (r *redisFileFill) Read(p []byte) (int, error) {
	n, err := r.source.Read(p)
	r.read += int64(n)
	if r.disabled || r.published {
		return n, err
	}
	if writeErr := r.appendChunks(p[:n]); writeErr != nil {
		r.discard(writeErr)
		return n, err
	}
	if err == nil {
		return n, nil
	}
	if err != io.EOF || r.read != r.expected || r.ctx.Err() != nil {
		r.discard(nil)
		return n, err
	}
	if publishErr := r.publish(); publishErr != nil {
		r.discard(publishErr)
	}
	return n, err
}

func (r *redisFileFill) appendChunks(data []byte) error {
	for len(data) > 0 {
		count := min(len(data), cap(r.buffer)-len(r.buffer))
		r.buffer = append(r.buffer, data[:count]...)
		data = data[count:]
		if len(r.buffer) == fileCacheChunkSize {
			if err := r.flush(); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *redisFileFill) publish() error {
	if err := r.flush(); err != nil {
		return err
	}
	published, err := publishFileScript.Run(r.ctx, r.cache.client, []string{r.stage, r.key}, r.chunks, r.expected, r.cache.ttl.Milliseconds()).Int()
	if err != nil {
		return err
	}
	if published != 1 {
		return errors.New("file cache fill expired before publication")
	}
	r.published = true
	r.buffer = nil
	return nil
}

func (r *redisFileFill) Close() error { r.discard(nil); return r.source.Close() }
