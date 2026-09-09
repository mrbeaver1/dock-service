package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/mrbeaver1/dock-service/internal/models"
)

type DeletionRepository interface {
	Claim(context.Context) (models.Deletion, bool, error)
	Complete(context.Context, models.Deletion) error
	Retry(context.Context, models.Deletion) error
}

type DeletionStorage interface {
	Delete(context.Context, string) error
}

type DeletionCache interface {
	InvalidateDocument(context.Context, models.Document) error
}

type DeletionCleanupService interface {
	Run(context.Context)
	ReconcileNext(context.Context) (bool, error)
}

type deletionCleanupService struct {
	repo   DeletionRepository
	source DeletionStorage
	cache  DeletionCache
}

func NewDeletionCleanupService(repo DeletionRepository, source DeletionStorage, cache DeletionCache) DeletionCleanupService {
	return &deletionCleanupService{repo: repo, source: source, cache: cache}
}

func (s *deletionCleanupService) Run(ctx context.Context) {
	runCleanup(ctx, "document deletion", s.ReconcileNext)
}

func (s *deletionCleanupService) ReconcileNext(ctx context.Context) (bool, error) {
	operation, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	job, found, err := s.repo.Claim(operation)
	if err != nil || !found {
		return found, err
	}
	if job.ObjectKey != nil {
		err = s.source.Delete(operation, *job.ObjectKey)
	}
	if err == nil {
		err = s.repo.Complete(operation, job)
	}
	cleanup, stop := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer stop()
	if err != nil {
		err = errors.Join(err, s.repo.Retry(cleanup, job))
	}
	document := models.Document{ID: job.ID, OwnerID: job.OwnerID, Generation: job.Generation, Version: job.Version}
	if cacheErr := s.cache.InvalidateDocument(cleanup, document); cacheErr != nil {
		slog.WarnContext(ctx, "invalidate deleted document", "document_id", job.ID, "error", cacheErr)
	}
	return true, err
}
