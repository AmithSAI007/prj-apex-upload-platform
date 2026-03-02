package service

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	"go.uber.org/zap"
)

// TestNewFirestoreUploadSessionStore_Constructor verifies the constructor
// populates all fields correctly.
func TestNewFirestoreUploadSessionStore_Constructor(t *testing.T) {
	logger := zap.NewNop()
	tracer := otel.Tracer("test")
	store := NewFirestoreUploadSessionStore(nil, logger, tracer)
	if store == nil {
		t.Fatal("expected non-nil store")
	}
	if store.logger != logger {
		t.Fatal("expected logger to be set")
	}
	if store.tracer != tracer {
		t.Fatal("expected tracer to be set")
	}
	// client is nil because we don't have a real Firestore client in tests.
	if store.client != nil {
		t.Fatal("expected nil client")
	}
}

// TestFirestoreUploadSessionStore_Create_NilSession verifies that Create
// returns an error when called with a nil session.
func TestFirestoreUploadSessionStore_Create_NilSession(t *testing.T) {
	store := NewFirestoreUploadSessionStore(nil, zap.NewNop(), otel.Tracer("test"))
	err := store.Create(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil session")
	}
	if err.Error() != "session is required" {
		t.Fatalf("unexpected error message: %v", err)
	}
}

// TestFirestoreUploadSessionStore_GetByIdempotencyKey_EmptyKey verifies that
// GetByIdempotencyKey returns (nil, nil) for an empty idempotency key.
func TestFirestoreUploadSessionStore_GetByIdempotencyKey_EmptyKey(t *testing.T) {
	store := NewFirestoreUploadSessionStore(nil, zap.NewNop(), otel.Tracer("test"))
	session, err := store.GetByIdempotencyKey(context.Background(), "tenant", "user", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session != nil {
		t.Fatal("expected nil session for empty idempotency key")
	}
}

// TestUploadSessionStoreInterface ensures FirestoreUploadSessionStore
// satisfies the UploadSessionStore interface at compile time.
func TestUploadSessionStoreInterface(t *testing.T) {
	var _ UploadSessionStore = (*FirestoreUploadSessionStore)(nil)
}
