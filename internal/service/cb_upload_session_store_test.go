package service

import (
	"context"
	"errors"
	"testing"
	"time"

	internalerrors "github.com/AmithSAI007/prj-apex-upload-platform/internal/errors"
	"github.com/AmithSAI007/prj-apex-upload-platform/internal/model"
	"github.com/sony/gobreaker/v2"
)

// cbTestStore is a minimal fake for UploadSessionStore used in CB tests.
type cbTestStore struct {
	createErr       error
	getByIDSession  *model.UploadSession
	getByIDErr      error
	idempSession    *model.UploadSession
	idempErr        error
	updateStatusErr error
	updateURLErr    error
	markCompErr     error
	markCancelErr   error
	markExpireErr   error
}

func (s *cbTestStore) Create(_ context.Context, _ *model.UploadSession) error { return s.createErr }
func (s *cbTestStore) GetByID(_ context.Context, _ string) (*model.UploadSession, error) {
	return s.getByIDSession, s.getByIDErr
}
func (s *cbTestStore) GetByIdempotencyKey(_ context.Context, _, _, _ string) (*model.UploadSession, error) {
	return s.idempSession, s.idempErr
}
func (s *cbTestStore) UpdateStatus(_ context.Context, _ string, _ model.UploadStatus, _ int64) error {
	return s.updateStatusErr
}
func (s *cbTestStore) UpdateGCSUploadURL(_ context.Context, _ string, _ string) error {
	return s.updateURLErr
}
func (s *cbTestStore) MarkCompleted(_ context.Context, _ string, _ int64) error {
	return s.markCompErr
}
func (s *cbTestStore) MarkCancelled(_ context.Context, _ string) error { return s.markCancelErr }
func (s *cbTestStore) MarkExpired(_ context.Context, _ string) error   { return s.markExpireErr }

// tripImmediately returns gobreaker.Settings that trip after 1 consecutive failure.
func tripImmediately() gobreaker.Settings {
	return gobreaker.Settings{
		Name: "test-store-cb",
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 1
		},
	}
}

func TestCBUploadSessionStore_Interface(t *testing.T) {
	var _ UploadSessionStore = (*CBUploadSessionStore)(nil)
}

func TestCBUploadSessionStore_Create_PassThrough(t *testing.T) {
	inner := &cbTestStore{}
	cb := NewCBUploadSessionStore(inner, tripImmediately())
	err := cb.Create(context.Background(), &model.UploadSession{})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestCBUploadSessionStore_Create_InnerError(t *testing.T) {
	inner := &cbTestStore{createErr: errors.New("firestore down")}
	cb := NewCBUploadSessionStore(inner, tripImmediately())
	err := cb.Create(context.Background(), &model.UploadSession{})
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "firestore down" {
		t.Fatalf("expected inner error, got %v", err)
	}
}

func TestCBUploadSessionStore_Create_CircuitOpen(t *testing.T) {
	inner := &cbTestStore{createErr: errors.New("transient")}
	cb := NewCBUploadSessionStore(inner, tripImmediately())

	// First call fails and trips the breaker.
	_ = cb.Create(context.Background(), &model.UploadSession{})

	// Second call should be rejected by the open circuit.
	err := cb.Create(context.Background(), &model.UploadSession{})
	if err == nil {
		t.Fatal("expected circuit open error")
	}
	if !errors.Is(err, internalerrors.ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}
}

func TestCBUploadSessionStore_GetByID_PassThrough(t *testing.T) {
	session := &model.UploadSession{UploadID: "u1"}
	inner := &cbTestStore{getByIDSession: session}
	cb := NewCBUploadSessionStore(inner, tripImmediately())

	result, err := cb.GetByID(context.Background(), "u1")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if result == nil || result.UploadID != "u1" {
		t.Fatalf("expected session u1, got %v", result)
	}
}

func TestCBUploadSessionStore_GetByID_NilResult(t *testing.T) {
	inner := &cbTestStore{getByIDSession: nil, getByIDErr: nil}
	cb := NewCBUploadSessionStore(inner, tripImmediately())

	result, err := cb.GetByID(context.Background(), "u1")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil result, got %v", result)
	}
}

func TestCBUploadSessionStore_GetByID_CircuitOpen(t *testing.T) {
	inner := &cbTestStore{getByIDErr: errors.New("transient")}
	cb := NewCBUploadSessionStore(inner, tripImmediately())

	// Trip the breaker.
	_, _ = cb.GetByID(context.Background(), "u1")

	// Should get circuit open.
	_, err := cb.GetByID(context.Background(), "u1")
	if !errors.Is(err, internalerrors.ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}
}

func TestCBUploadSessionStore_GetByIdempotencyKey_PassThrough(t *testing.T) {
	session := &model.UploadSession{UploadID: "u2"}
	inner := &cbTestStore{idempSession: session}
	cb := NewCBUploadSessionStore(inner, tripImmediately())

	result, err := cb.GetByIdempotencyKey(context.Background(), "t", "u", "key")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if result == nil || result.UploadID != "u2" {
		t.Fatalf("expected session u2, got %v", result)
	}
}

func TestCBUploadSessionStore_GetByIdempotencyKey_NilResult(t *testing.T) {
	inner := &cbTestStore{}
	cb := NewCBUploadSessionStore(inner, tripImmediately())

	result, err := cb.GetByIdempotencyKey(context.Background(), "t", "u", "key")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil result, got %v", result)
	}
}

func TestCBUploadSessionStore_UpdateStatus_CircuitOpen(t *testing.T) {
	inner := &cbTestStore{updateStatusErr: errors.New("transient")}
	cb := NewCBUploadSessionStore(inner, tripImmediately())

	_ = cb.UpdateStatus(context.Background(), "u1", model.StatusInProgress, 0)

	err := cb.UpdateStatus(context.Background(), "u1", model.StatusInProgress, 0)
	if !errors.Is(err, internalerrors.ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}
}

func TestCBUploadSessionStore_UpdateGCSUploadURL_CircuitOpen(t *testing.T) {
	inner := &cbTestStore{updateURLErr: errors.New("transient")}
	cb := NewCBUploadSessionStore(inner, tripImmediately())

	_ = cb.UpdateGCSUploadURL(context.Background(), "u1", "https://example.com")

	err := cb.UpdateGCSUploadURL(context.Background(), "u1", "https://example.com")
	if !errors.Is(err, internalerrors.ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}
}

func TestCBUploadSessionStore_MarkCompleted_CircuitOpen(t *testing.T) {
	inner := &cbTestStore{markCompErr: errors.New("transient")}
	cb := NewCBUploadSessionStore(inner, tripImmediately())

	_ = cb.MarkCompleted(context.Background(), "u1", 1024)

	err := cb.MarkCompleted(context.Background(), "u1", 1024)
	if !errors.Is(err, internalerrors.ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}
}

func TestCBUploadSessionStore_MarkCancelled_CircuitOpen(t *testing.T) {
	inner := &cbTestStore{markCancelErr: errors.New("transient")}
	cb := NewCBUploadSessionStore(inner, tripImmediately())

	_ = cb.MarkCancelled(context.Background(), "u1")

	err := cb.MarkCancelled(context.Background(), "u1")
	if !errors.Is(err, internalerrors.ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}
}

func TestCBUploadSessionStore_MarkExpired_CircuitOpen(t *testing.T) {
	inner := &cbTestStore{markExpireErr: errors.New("transient")}
	cb := NewCBUploadSessionStore(inner, tripImmediately())

	_ = cb.MarkExpired(context.Background(), "u1")

	err := cb.MarkExpired(context.Background(), "u1")
	if !errors.Is(err, internalerrors.ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}
}

func TestCBUploadSessionStore_Recovery(t *testing.T) {
	callCount := 0
	inner := &cbTestStore{}

	// Use settings that allow recovery: trip after 1 failure, very short
	// timeout so it transitions to half-open quickly, allow 1 request.
	settings := gobreaker.Settings{
		Name:        "test-recovery",
		MaxRequests: 1,
		Timeout:     1 * time.Millisecond,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 1
		},
	}
	cb := NewCBUploadSessionStore(inner, settings)

	// Trip the breaker with a failure.
	inner.createErr = errors.New("fail")
	_ = cb.Create(context.Background(), &model.UploadSession{})
	callCount++

	// Fix the inner store — subsequent calls should succeed after timeout.
	inner.createErr = nil

	// Wait for the breaker timeout to elapse so it transitions to half-open.
	time.Sleep(5 * time.Millisecond)

	err := cb.Create(context.Background(), &model.UploadSession{})
	callCount++
	if err != nil {
		t.Fatalf("expected recovery after timeout, got %v (calls: %d)", err, callCount)
	}
}
