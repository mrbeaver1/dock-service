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
