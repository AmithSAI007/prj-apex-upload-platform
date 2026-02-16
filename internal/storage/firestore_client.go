package storage

import (
	"context"
	"fmt"

	"cloud.google.com/go/firestore"
)

// FirestoreClient wraps the Firestore SDK client for shared use.
type FirestoreClient struct {
	client *firestore.Client
}

// NewFirestoreClient initializes a Firestore client for the project.
func NewFirestoreClient(ctx context.Context, projectID string, databaseID string) (*FirestoreClient, error) {
	if databaseID == "" {
		return nil, fmt.Errorf("databaseID cannot be empty")
	}
	client, err := firestore.NewClientWithDatabase(ctx, projectID, databaseID)
	if err != nil {
		return nil, err
	}

	return &FirestoreClient{client: client}, nil
}

// Client returns the underlying Firestore client.
func (c *FirestoreClient) Client() *firestore.Client {
	return c.client
}

// Close releases resources held by the Firestore client.
func (c *FirestoreClient) Close() error {
	if c == nil || c.client == nil {
		return nil
	}
	return c.client.Close()
}
