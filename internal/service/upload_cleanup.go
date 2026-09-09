package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/mrbeaver1/dock-service/internal/models"
)

type UploadRepository interface {
	Create(context.Context, models.Upload, time.Duration) error
	Schedule(context.Context, uuid.UUID) error
	Claim(context.Context) (models.Upload, bool, error)
	Complete(context.Context, models.Upload) error
	Retry(context.Context, models.Upload) error
}

type UploadStorage interface {
	AbortUpload(context.Context, string) error
	Delete(context.Context, string) error
}

type UploadDocuments interface {
	GetByID(context.Context, uuid.UUID) (models.Document, error)
}

type UploadCleanupService interface {
	Run(context.Context)
	ReconcileNext(context.Context) (bool, error)
}

type uploadCleanupService struct {
	repo      UploadRepository
	documents UploadDocuments
	source    UploadStorage
}

func NewUploadCleanupService(repo UploadRepository, documents UploadDocuments, source UploadStorage) UploadCleanupService {
	return &uploadCleanupService{repo: repo, documents: documents, source: source}
}

func (s *uploadCleanupService) Run(ctx context.Context) {
	runCleanup(ctx, "upload", s.ReconcileNext)
}

func (s *uploadCleanupService) ReconcileNext(ctx context.Context) (bool, error) {
	operation, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	upload, found, err := s.repo.Claim(operation)
	if err != nil || !found {
		return found, err
	}
	err = s.reconcile(operation, upload)
	if err != nil {
		retry, stop := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer stop()
		err = errors.Join(err, s.repo.Retry(retry, upload))
	}
	return true, err
}

func (s *uploadCleanupService) reconcile(ctx context.Context, upload models.Upload) error {
	doc, err := s.documents.GetByID(ctx, upload.ID)
	if err != nil && !errors.Is(err, models.ErrDocumentNotFound) {
		return err
	}
	if err == nil {
		if doc.OwnerID != upload.OwnerID || doc.ObjectKey == nil || *doc.ObjectKey != upload.ObjectKey {
			return errors.New("upload recovery document does not match the recorded object")
		}
		return s.repo.Complete(ctx, upload)
	}
	abortErr := s.source.AbortUpload(ctx, upload.ObjectKey)
	deleteErr := s.source.Delete(ctx, upload.ObjectKey)
	if err := errors.Join(abortErr, deleteErr); err != nil {
		return err
	}
	return s.repo.Complete(ctx, upload)
}
