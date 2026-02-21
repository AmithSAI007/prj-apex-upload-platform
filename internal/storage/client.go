package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"cloud.google.com/go/storage"
)

// SignedURLClient defines signed URL creation behavior.
type SignedURLClient interface {
	SignResumableUploadURL(bucket string, objectName string, serviceAccount string) (string, error)
}

// GCSClient wraps the GCS SDK client for shared use.
type GCSClient struct {
	client *storage.Client
}

// SignedURLOptions configures resumable upload URL signing.
type SignedURLOptions struct {
	ExpiresAt   time.Time
	ContentType string
}

// NewGCSClient initializes a GCS client with ADC.
func NewGCSClient(ctx context.Context) (*GCSClient, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, err
	}

	return &GCSClient{client: client}, nil
}

// Client returns the underlying storage client.
func (c *GCSClient) Client() *storage.Client {
	return c.client
}

// SignResumableUploadURL creates a signed URL for resumable uploads.
func (c *GCSClient) SignResumableUploadURL(bucket string, objectName string, serviceAccount string) (string, error) {
	if bucket == "" || objectName == "" || serviceAccount == "" {
		return "", errors.New("bucket and object name are required")
	}

	opts := &storage.SignedURLOptions{
		GoogleAccessID: serviceAccount,
		Scheme:         storage.SigningSchemeV4,
		Method:         "PUT",
		Headers: []string{
			"Content-Type:application/octet-stream",
		},
		Expires: time.Now().Add(15 * time.Minute),
	}

	u, err := c.client.Bucket(bucket).SignedURL(objectName, opts)
	if err != nil {
		return "", fmt.Errorf("Bucket(%q).SignedURL: %w", bucket, err)
	}

	return u, nil
}

// Close releases resources held by the GCS client.
func (c *GCSClient) Close() error {
	if c == nil || c.client == nil {
		return nil
	}
	return c.client.Close()
}
