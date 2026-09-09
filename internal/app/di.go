package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"github.com/mrbeaver1/dock-service/internal/api"
	"github.com/mrbeaver1/dock-service/internal/api/middleware"
	"github.com/mrbeaver1/dock-service/internal/config"
	"github.com/mrbeaver1/dock-service/internal/repository"
	"github.com/mrbeaver1/dock-service/internal/service"
	"github.com/mrbeaver1/dock-service/internal/storage/cache"
	"github.com/mrbeaver1/dock-service/internal/storage/db"
	"github.com/mrbeaver1/dock-service/internal/storage/s3"
	"github.com/mrbeaver1/dock-service/internal/storage/staging"
	"github.com/mrbeaver1/dock-service/migrations"
	goredis "github.com/redis/go-redis/v9"
	"golang.org/x/time/rate"
)

type diContainer struct {
	cnf      config.Config
	postgres *pgxpool.Pool
	redis    *goredis.Client
	s3       *minio.Client
	closeS3  func()
	staging  staging.Directory

	userService              service.UserService
	userServiceOnce          sync.Once
	userServiceErr           error
	passwordHasher           service.PasswordHasher
	passwordHasherOnce       sync.Once
	passwordHasherErr        error
	uploadLimiter            *middleware.BodyLimiter
	uploadLimiterOnce        sync.Once
	uploadLimiterErr         error
	credentialLimiter        *middleware.BodyLimiter
	credentialLimiterOnce    sync.Once
	credentialLimiterErr     error
	uploadRepo               repository.UploadRepository
	uploadRepoOnce           sync.Once
	uploadCleanupService     service.UploadCleanupService
	uploadCleanupServiceOnce sync.Once
	deletionRepo             repository.DeletionRepository
	deletionRepoOnce         sync.Once
	deletionCleanupService   service.DeletionCleanupService
	deletionCleanupOnce      sync.Once
	deletionCleanupErr       error
	tokenIssuer              service.TokenIssuer
	tokenIssuerOnce          sync.Once
	tokenIssuerErr           error
	userRepo                 repository.UserRepository
	userRepoOnce             sync.Once
	sessionRepo              repository.SessionRepository
	sessionRepoOnce          sync.Once
	sessionService           service.SessionService
	sessionServiceOnce       sync.Once
	sessionServiceErr        error
	documentRepo             service.DocumentRepository
	documentRepoOnce         sync.Once
	documentService          service.DocumentService
	documentServiceOnce      sync.Once
	documentServiceErr       error
	cacheService             service.Cache
	cacheServiceOnce         sync.Once
	cacheServiceErr          error
	s3Service                service.S3
	s3ServiceOnce            sync.Once
	handler                  http.Handler
	handlerOnce              sync.Once
	handlerErr               error

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

	d.staging, err = staging.NewDirectory(cfg.Upload.TempDir)
	if err != nil {
		return nil, fmt.Errorf("initialize multipart staging: %w", err)
	}

	d.postgres, err = db.NewPool(ctx, cfg.Postgres)
	if err != nil {
		return nil, fmt.Errorf("initialize PostgreSQL: %w", err)
	}
	if err = migrations.Check(ctx, d.postgres); err != nil {
		return nil, err
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

func (d *diContainer) UserService() (service.UserService, error) {
	d.userServiceOnce.Do(func() {
		tokens, err := d.TokenIssuer()
		if err != nil {
			d.userServiceErr = err
			return
		}
		passwords, err := d.PasswordHasher()
		if err != nil {
			d.userServiceErr = err
			return
		}
		d.userService = service.NewUserService(d.UserRepository(), passwords, tokens)
	})

	return d.userService, d.userServiceErr
}

func (d *diContainer) PasswordHasher() (service.PasswordHasher, error) {
	d.passwordHasherOnce.Do(func() { d.passwordHasher, d.passwordHasherErr = service.NewPasswordHasher(d.cnf.PasswordConcurrency) })
	return d.passwordHasher, d.passwordHasherErr
}

func (d *diContainer) UploadLimiter() (*middleware.BodyLimiter, error) {
	d.uploadLimiterOnce.Do(func() {
		cfg := d.cnf.Upload
		d.uploadLimiter, d.uploadLimiterErr = middleware.NewBodyLimiter(cfg.Concurrent, cfg.MaxBytes, cfg.IdleTimeout, cfg.ReadTimeout)
	})
	return d.uploadLimiter, d.uploadLimiterErr
}

func (d *diContainer) CredentialLimiter() (*middleware.BodyLimiter, error) {
	d.credentialLimiterOnce.Do(func() {
		cfg := d.cnf.Credentials
		d.credentialLimiter, d.credentialLimiterErr = middleware.NewBodyLimiter(cfg.Concurrent, cfg.MaxBytes, cfg.IdleTimeout, cfg.ReadTimeout)
	})
	return d.credentialLimiter, d.credentialLimiterErr
}

func (d *diContainer) UploadRepository() repository.UploadRepository {
	d.uploadRepoOnce.Do(func() { d.uploadRepo = repository.NewUploadRepository(d.postgres) })
	return d.uploadRepo
}

func (d *diContainer) UploadCleanupService() service.UploadCleanupService {
	d.uploadCleanupServiceOnce.Do(func() {
		d.uploadCleanupService = service.NewUploadCleanupService(d.UploadRepository(), d.DocumentRepository(), d.S3Service())
	})
	return d.uploadCleanupService
}

func (d *diContainer) DeletionRepository() repository.DeletionRepository {
	d.deletionRepoOnce.Do(func() { d.deletionRepo = repository.NewDeletionRepository(d.postgres) })
	return d.deletionRepo
}

func (d *diContainer) DeletionCleanupService() (service.DeletionCleanupService, error) {
	d.deletionCleanupOnce.Do(func() {
		cache, err := d.CacheService()
		if err != nil {
			d.deletionCleanupErr = err
			return
		}
		d.deletionCleanupService = service.NewDeletionCleanupService(d.DeletionRepository(), d.S3Service(), cache)
	})
	return d.deletionCleanupService, d.deletionCleanupErr
}

func (d *diContainer) TokenIssuer() (service.TokenIssuer, error) {
	d.tokenIssuerOnce.Do(func() { d.tokenIssuer, d.tokenIssuerErr = service.NewJWTIssuer([]byte(d.cnf.JWTSecret)) })
	return d.tokenIssuer, d.tokenIssuerErr
}

func (d *diContainer) UserRepository() repository.UserRepository {
	d.userRepoOnce.Do(func() {
		d.userRepo = repository.NewUserRepository(d.postgres)
	})

	return d.userRepo
}

func (d *diContainer) Cnf() config.Config {
	return d.cnf
}

func (d *diContainer) SessionRepository() repository.SessionRepository {
	d.sessionRepoOnce.Do(func() { d.sessionRepo = repository.NewSessionRepository(d.postgres) })
	return d.sessionRepo
}

func (d *diContainer) SessionService() (service.SessionService, error) {
	d.sessionServiceOnce.Do(func() {
		d.sessionService, d.sessionServiceErr = service.NewSessionService(d.SessionRepository(), []byte(d.cnf.JWTSecret))
	})
	return d.sessionService, d.sessionServiceErr
}

func (d *diContainer) DocumentRepository() service.DocumentRepository {
	d.documentRepoOnce.Do(func() { d.documentRepo = repository.NewDocumentRepository(d.postgres) })
	return d.documentRepo
}

func (d *diContainer) CacheService() (service.Cache, error) {
	d.cacheServiceOnce.Do(func() { d.cacheService, d.cacheServiceErr = service.NewCache(d.redis, d.cnf.CacheTTL) })
	return d.cacheService, d.cacheServiceErr
}

func (d *diContainer) S3Service() service.S3 {
	d.s3ServiceOnce.Do(func() { d.s3Service = service.NewS3(d.s3, d.cnf.S3.Bucket) })
	return d.s3Service
}

func (d *diContainer) DocumentService() (service.DocumentService, error) {
	d.documentServiceOnce.Do(func() {
		cache, err := d.CacheService()
		if err != nil {
			d.documentServiceErr = err
			return
		}
		d.documentService = service.NewDocumentService(d.DocumentRepository(), d.S3Service(), cache, d.UserRepository(), d.UploadRepository(), d.cnf.Upload.Timeout)
	})
	return d.documentService, d.documentServiceErr
}

func (d *diContainer) Handler() (http.Handler, error) {
	d.handlerOnce.Do(func() {
		documents, err := d.DocumentService()
		if err != nil {
			d.handlerErr = err
			return
		}
		users, err := d.UserService()
		if err != nil {
			d.handlerErr = err
			return
		}
		sessions, err := d.SessionService()
		if err != nil {
			d.handlerErr = err
			return
		}
		limiter := middleware.NewIPLimiter(rate.Limit(d.cnf.RPS), int(d.cnf.Burst))
		uploads, err := d.UploadLimiter()
		if err != nil {
			d.handlerErr = err
			return
		}
		credentials, err := d.CredentialLimiter()
		if err != nil {
			d.handlerErr = err
			return
		}
		d.handler = api.NewHandler(documents, users, sessions, limiter, uploads, credentials, d.cnf.AdminToken).Routes()
	})
	return d.handler, d.handlerErr
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
		if d.staging != nil {
			d.closeErr = errors.Join(d.closeErr, d.staging.Close())
		}
	})
	return d.closeErr
}
