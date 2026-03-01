package storage

import (
	"context"
	"fmt"

	"cloud.google.com/go/firestore"
)

// FirestoreClient wraps the Firestore SDK client for shared use across the
// application. It is initialized once at startup and passed to service
// constructors that need Firestore access.
type FirestoreClient struct {
	client *firestore.Client
}

// NewFirestoreClient initializes a Firestore client for the given GCP project
// and named database. The databaseID parameter is required and must reference
// an existing Firestore database. Returns an error if databaseID is empty or
// the Firestore connection cannot be established.
func NewFirestoreClient(ctx context.Context, projectID string, databaseID string) (*FirestoreClient, error) {
	if databaseID == "" {
		return nil, fmt.Errorf("databaseID cannot be empty")
	}

	// Create a client targeting a specific named Firestore database (not the default).
	client, err := firestore.NewClientWithDatabase(ctx, projectID, databaseID)
	if err != nil {
		return nil, err
	}

	return &FirestoreClient{client: client}, nil
}

// Client returns the underlying Firestore SDK client for direct use by
// services that need collection/document operations.
func (c *FirestoreClient) Client() *firestore.Client {
	return c.client
}

// Close releases resources held by the Firestore gRPC client. Safe to call
// on nil receivers.
func (c *FirestoreClient) Close() error {
	if c == nil || c.client == nil {
		return nil
	}
	return c.client.Close()
}
