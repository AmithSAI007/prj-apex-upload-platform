package service

import (
	"context"
	"errors"
	"fmt"

	internalerrors "github.com/AmithSAI007/prj-apex-upload-platform/internal/errors"
	"github.com/AmithSAI007/prj-apex-upload-platform/internal/model"
	"github.com/sony/gobreaker/v2"
)

// CBUploadSessionStore is a circuit breaker decorator around an
// UploadSessionStore implementation. It delegates every call through a
// gobreaker CircuitBreaker, rejecting requests with ErrCircuitOpen when the
// breaker is open. The call chain is:
//
//	Service → CBUploadSessionStore → inner (FirestoreUploadSessionStore with retry)
type CBUploadSessionStore struct {
	inner UploadSessionStore
	cb    *gobreaker.CircuitBreaker[any]
}

// NewCBUploadSessionStore wraps inner with a circuit breaker using the given
// gobreaker settings. The caller is responsible for configuring ReadyToTrip,
// OnStateChange, and other breaker policies via the settings.
func NewCBUploadSessionStore(inner UploadSessionStore, settings gobreaker.Settings) *CBUploadSessionStore {
	return &CBUploadSessionStore{
		inner: inner,
		cb:    gobreaker.NewCircuitBreaker[any](settings),
	}
}

// wrapCBError converts a gobreaker.ErrOpenState or ErrTooManyRequests into
// the application-level ErrCircuitOpen sentinel for consistent HTTP mapping.
func wrapCBError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gobreaker.ErrOpenState) || errors.Is(err, gobreaker.ErrTooManyRequests) {
		return fmt.Errorf("%w: %w", internalerrors.ErrCircuitOpen, err)
	}
	return err
}

func (s *CBUploadSessionStore) Create(ctx context.Context, session *model.UploadSession) error {
	_, err := s.cb.Execute(func() (any, error) {
		return nil, s.inner.Create(ctx, session)
	})
	return wrapCBError(err)
}

func (s *CBUploadSessionStore) GetByID(ctx context.Context, uploadID string) (*model.UploadSession, error) {
	result, err := s.cb.Execute(func() (any, error) {
		return s.inner.GetByID(ctx, uploadID)
	})
	if err != nil {
		return nil, wrapCBError(err)
	}
	if result == nil {
		return nil, nil
	}
	return result.(*model.UploadSession), nil
}

func (s *CBUploadSessionStore) GetByIdempotencyKey(ctx context.Context, tenantID string, userID string, idempotencyKey string) (*model.UploadSession, error) {
	result, err := s.cb.Execute(func() (any, error) {
		return s.inner.GetByIdempotencyKey(ctx, tenantID, userID, idempotencyKey)
	})
	if err != nil {
		return nil, wrapCBError(err)
	}
	if result == nil {
		return nil, nil
	}
	return result.(*model.UploadSession), nil
}

func (s *CBUploadSessionStore) UpdateStatus(ctx context.Context, uploadID string, status model.UploadStatus, uploadedBytes int64) error {
	_, err := s.cb.Execute(func() (any, error) {
		return nil, s.inner.UpdateStatus(ctx, uploadID, status, uploadedBytes)
	})
	return wrapCBError(err)
}

func (s *CBUploadSessionStore) UpdateGCSUploadURL(ctx context.Context, uploadID string, gcsUploadURL string) error {
	_, err := s.cb.Execute(func() (any, error) {
		return nil, s.inner.UpdateGCSUploadURL(ctx, uploadID, gcsUploadURL)
	})
	return wrapCBError(err)
}

func (s *CBUploadSessionStore) MarkCompleted(ctx context.Context, uploadID string, uploadedBytes int64) error {
	_, err := s.cb.Execute(func() (any, error) {
		return nil, s.inner.MarkCompleted(ctx, uploadID, uploadedBytes)
	})
	return wrapCBError(err)
}

func (s *CBUploadSessionStore) MarkCancelled(ctx context.Context, uploadID string) error {
	_, err := s.cb.Execute(func() (any, error) {
		return nil, s.inner.MarkCancelled(ctx, uploadID)
	})
	return wrapCBError(err)
}

func (s *CBUploadSessionStore) MarkExpired(ctx context.Context, uploadID string) error {
	_, err := s.cb.Execute(func() (any, error) {
		return nil, s.inner.MarkExpired(ctx, uploadID)
	})
	return wrapCBError(err)
}

var _ UploadSessionStore = (*CBUploadSessionStore)(nil)
