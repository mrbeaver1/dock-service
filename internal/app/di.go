package app

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"github.com/mrbeaver1/dock-service/internal/config"
	"github.com/mrbeaver1/dock-service/internal/repository"
	"github.com/mrbeaver1/dock-service/internal/service"
	"github.com/mrbeaver1/dock-service/internal/storage/cache"
	"github.com/mrbeaver1/dock-service/internal/storage/db"
	"github.com/mrbeaver1/dock-service/internal/storage/s3"
	goredis "github.com/redis/go-redis/v9"
)

type diContainer struct {
	cnf      config.Config
	postgres *pgxpool.Pool
	redis    *goredis.Client
	s3       *minio.Client
	closeS3  func()

	userService     service.UserService
	userServiceOnce sync.Once
	userRepo        service.UserRepository
	userRepoOnce    sync.Once

	closeOnce sync.Once
	closeErr  error
}

func newDiContainer(ctx context.Context) (_ *diContainer, err error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("load configuration: %w", err)
	}

	d := &diContainer{cnf: cfg}
	defer func() {
		if err != nil {
			err = errors.Join(err, d.Close())
		}
	}()

	d.postgres, err = db.NewPool(ctx, cfg.Postgres)
	if err != nil {
		return nil, fmt.Errorf("initialize PostgreSQL: %w", err)
	}

	d.redis, err = cache.NewClient(ctx, cfg.Redis)
	if err != nil {
		return nil, fmt.Errorf("initialize Redis: %w", err)
	}

	d.s3, d.closeS3, err = s3.NewClient(ctx, cfg.S3)
	if err != nil {
		return nil, fmt.Errorf("initialize S3: %w", err)
	}

	return d, nil
}

func (d *diContainer) UserService() service.UserService {
	d.userServiceOnce.Do(func() {
		d.userService = service.NewUserService(d.UserRepository())
	})

	return d.userService
}

func (d *diContainer) UserRepository() service.UserRepository {
	d.userRepoOnce.Do(func() {
		d.userRepo = repository.NewUserRepository(d.postgres)
	})

	return d.userRepo
}

func (d *diContainer) Cnf() config.Config {
	return d.cnf
}

func (d *diContainer) Postgres() *pgxpool.Pool {
	return d.postgres
}

func (d *diContainer) Redis() *goredis.Client {
	return d.redis
}

func (d *diContainer) S3() *minio.Client {
	return d.s3
}

func (d *diContainer) Close() error {
	d.closeOnce.Do(func() {
		if d.closeS3 != nil {
			d.closeS3()
		}
		if d.redis != nil {
			if err := d.redis.Close(); err != nil {
				d.closeErr = fmt.Errorf("close Redis: %w", err)
			}
		}
		if d.postgres != nil {
			d.postgres.Close()
		}
	})
	return d.closeErr
}
