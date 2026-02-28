package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"cloud.google.com/go/storage"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// SignedURLClient defines signed URL creation behavior.
type SignedURLClient interface {
	SignResumableUploadURL(ctx context.Context, bucket string, objectName string, serviceAccount string) (string, error)
}

// GCSClient wraps the GCS SDK client for shared use.
type GCSClient struct {
	client *storage.Client
	trace  trace.Tracer
}

// SignedURLOptions configures resumable upload URL signing.
type SignedURLOptions struct {
	ExpiresAt   time.Time
	ContentType string
}

// NewGCSClient initializes a GCS client with ADC.
func NewGCSClient(ctx context.Context, trace trace.Tracer) (*GCSClient, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, err
	}

	return &GCSClient{client: client, trace: trace}, nil
}

// Client returns the underlying storage client.
func (c *GCSClient) Client() *storage.Client {
	return c.client
}

// SignResumableUploadURL creates a signed URL for resumable uploads.
func (c *GCSClient) SignResumableUploadURL(ctx context.Context, bucket string, objectName string, serviceAccount string) (string, error) {

	ctx, span := c.trace.Start(ctx, "SignResumableUploadURL")
	defer span.End()

	if bucket == "" || objectName == "" || serviceAccount == "" {
		span.RecordError(errors.New("bucket, object name, and service account are required"))
		span.SetStatus(codes.Error, "Invalid input parameters")
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
		span.RecordError(fmt.Errorf("failed to sign URL: %w", err))
		span.SetStatus(codes.Error, "Failed to sign URL")
		return "", fmt.Errorf("Bucket(%q).SignedURL: %w", bucket, err)
	}

	span.SetStatus(codes.Ok, "Successfully signed URL")
	return u, nil
}

// Close releases resources held by the GCS client.
func (c *GCSClient) Close() error {
	if c == nil || c.client == nil {
		return nil
	}
	return c.client.Close()
}
