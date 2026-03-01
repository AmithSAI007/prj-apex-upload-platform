package storage

import (
	"context"
	"testing"
)

// TestNewFirestoreClient_EmptyDatabaseID verifies that the constructor rejects
// an empty databaseID with a descriptive error.
func TestNewFirestoreClient_EmptyDatabaseID(t *testing.T) {
	_, err := NewFirestoreClient(context.Background(), "project-id", "")
	if err == nil {
		t.Fatal("expected error for empty databaseID")
	}
	if err.Error() != "databaseID cannot be empty" {
		t.Fatalf("unexpected error message: %v", err)
	}
}

// TestFirestoreClient_Close_NilReceiver verifies Close on a nil
// *FirestoreClient returns nil without panicking.
func TestFirestoreClient_Close_NilReceiver(t *testing.T) {
	var c *FirestoreClient
	if err := c.Close(); err != nil {
		t.Fatalf("expected nil error for nil receiver, got %v", err)
	}
}

// TestFirestoreClient_Close_NilInnerClient verifies Close returns nil when
// the inner firestore client is nil.
func TestFirestoreClient_Close_NilInnerClient(t *testing.T) {
	c := &FirestoreClient{client: nil}
	if err := c.Close(); err != nil {
		t.Fatalf("expected nil error for nil inner client, got %v", err)
	}
}

// TestFirestoreClient_Client_NilInner verifies the Client() accessor returns
// nil when no inner client is set.
func TestFirestoreClient_Client_NilInner(t *testing.T) {
	c := &FirestoreClient{client: nil}
	if c.Client() != nil {
		t.Fatal("expected nil inner client")
	}
}
