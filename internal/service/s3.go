package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/minio/minio-go/v7"
)

type S3 interface {
	Open(ctx context.Context, key string, offset int64) (io.ReadCloser, int64, error)
	Put(ctx context.Context, key string, content io.Reader, size int64, contentType string) error
	Delete(ctx context.Context, key string) error
	AbortUpload(context.Context, string) error
}

type s3 struct {
	client *minio.Client
	bucket string
}

func NewS3(client *minio.Client, bucket string) S3 { return &s3{client: client, bucket: bucket} }

func (s *s3) Open(ctx context.Context, key string, offset int64) (io.ReadCloser, int64, error) {
	opts := minio.GetObjectOptions{}
	if offset > 0 {
		if err := opts.SetRange(offset, 0); err != nil {
			return nil, 0, err
		}
	}
	core := minio.Core{Client: s.client}
	body, info, _, err := core.GetObject(ctx, s.bucket, key, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("open S3 object %q: %w", key, err)
	}
	return body, info.Size, nil
}

func (s *s3) Put(ctx context.Context, key string, content io.Reader, size int64, contentType string) error {
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	_, err := s.client.PutObject(ctx, s.bucket, key, content, size, minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		err = errors.Join(err, s.AbortUpload(cleanup, key))
		return fmt.Errorf("put S3 object %q: %w", key, err)
	}
	return nil
}

func (s *s3) AbortUpload(ctx context.Context, key string) error {
	if err := s.client.RemoveIncompleteUpload(ctx, s.bucket, key); err != nil {
		return fmt.Errorf("abort incomplete S3 upload %q: %w", key, err)
	}
	return nil
}

func (s *s3) Delete(ctx context.Context, key string) error {
	if err := s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("delete S3 object %q: %w", key, err)
	}
	return nil
}
