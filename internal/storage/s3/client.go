package s3

import (
	"context"
	"fmt"
	"net"
	"strconv"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/mrbeaver1/dock-service/internal/config"
)

// NewClient creates a client for the HTTP endpoint configured in Docker Compose
// and checks the bucket provisioned by s3-init. It does not create the bucket.
// The cleanup function must be called after all S3 operations have finished.
func NewClient(ctx context.Context, cfg config.S3Config) (*minio.Client, func(), error) {
	transport, err := minio.DefaultTransport(false)
	if err != nil {
		return nil, nil, fmt.Errorf("create S3 transport: %w", err)
	}

	client, err := minio.New(
		net.JoinHostPort(cfg.Host, strconv.Itoa(int(cfg.APIPort))),
		&minio.Options{
			Creds:     credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
			Secure:    false,
			Transport: transport,
		},
	)
	if err != nil {
		transport.CloseIdleConnections()
		return nil, nil, fmt.Errorf("create S3 client: %w", err)
	}

	exists, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		transport.CloseIdleConnections()
		return nil, nil, fmt.Errorf("check S3 bucket %q: %w", cfg.Bucket, err)
	}
	if !exists {
		transport.CloseIdleConnections()
		return nil, nil, fmt.Errorf("S3 bucket %q does not exist", cfg.Bucket)
	}

	return client, transport.CloseIdleConnections, nil
}
