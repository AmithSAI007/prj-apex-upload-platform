// Package storage provides GCS and Firestore client wrappers used by the
// upload service. The GCSClient generates signed resumable upload URLs so
// that web clients can upload directly to GCS without proxying data through
// this service.
package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"cloud.google.com/go/storage"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/AmithSAI007/prj-apex-upload-platform/internal/config"
	"github.com/AmithSAI007/prj-apex-upload-platform/internal/resilience"
	"github.com/cenkalti/backoff/v5"
)

// SignedURLClient defines the interface for generating signed GCS URLs.
// The upload service depends on this interface for testability.
type SignedURLClient interface {
	// SignResumableUploadURL creates a V4-signed PUT URL for the given bucket
	// and object name, authenticated as serviceAccount. The URL expires after
	// 15 minutes.
	SignResumableUploadURL(ctx context.Context, bucket string, objectName string, serviceAccount string) (string, error)
}

// GCSClient wraps the GCS SDK client for shared use.
type GCSClient struct {
	client *storage.Client
	trace  trace.Tracer
	cfg    *config.Config
}

// SignedURLOptions configures resumable upload URL signing.
type SignedURLOptions struct {
	ExpiresAt   time.Time
	ContentType string
}

// NewGCSClient initializes a GCS client with ADC.
func NewGCSClient(ctx context.Context, trace trace.Tracer, cfg *config.Config) (*GCSClient, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, err
	}

	return &GCSClient{client: client, trace: trace, cfg: cfg}, nil
}

// Client returns the underlying storage client.
func (c *GCSClient) Client() *storage.Client {
	return c.client
}

// SignResumableUploadURL creates a V4-signed URL for resumable uploads to GCS.
// It validates that bucket, objectName, and serviceAccount are all non-empty,
// then delegates to the GCS SDK's SignedURL method. Returns the signed URL or
// an error if validation or signing fails.
func (c *GCSClient) SignResumableUploadURL(ctx context.Context, bucket string, objectName string, serviceAccount string) (string, error) {

	_, span := c.trace.Start(ctx, "SignResumableUploadURL")
	defer span.End()

	span.AddEvent("sign_url.start", trace.WithAttributes(
		attribute.String("bucket", bucket),
		attribute.String("object_name", objectName),
	))

	if bucket == "" || objectName == "" || serviceAccount == "" {
		span.RecordError(errors.New("bucket, object name, and service account are required"))
		span.SetStatus(codes.Error, "Invalid input parameters")
		span.AddEvent("sign_url.invalid_input")
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

	operation := func() (string, error) {
		u, err := c.client.Bucket(bucket).SignedURL(objectName, opts)
		if err != nil {
			// Classify error: non-retryable GCS errors (4xx except 429) are
			// wrapped with backoff.Permanent to stop the retry loop immediately.
			if !resilience.IsRetryable(err) {
				return "", backoff.Permanent(fmt.Errorf("Bucket(%q).SignedURL: %w", bucket, err))
			}
			return "", fmt.Errorf("Bucket(%q).SignedURL: %w", bucket, err)
		}
		return u, nil
	}

	result, err := backoff.Retry(ctx, operation,
		backoff.WithBackOff(backoff.NewExponentialBackOff()),
		backoff.WithMaxElapsedTime(time.Duration(c.cfg.MaxElapsedTimeSeconds)*time.Second),
		backoff.WithMaxTries(uint(c.cfg.MaxRetryAttempts)))

	if err != nil {
		// Record span error only after the retry loop completes, not on every
		// attempt. This avoids polluting traces with transient failures that
		// are eventually retried successfully.
		span.RecordError(fmt.Errorf("failed to sign URL: %w", err))
		span.SetStatus(codes.Error, "Failed to sign URL")
		span.AddEvent("sign_url.failed", trace.WithAttributes(
			attribute.String("error", err.Error()),
		))
		return "", err
	}
	span.SetStatus(codes.Ok, "Successfully signed URL")
	span.AddEvent("sign_url.success")
	return result, nil
}

// Close releases resources held by the GCS client.
func (c *GCSClient) Close() error {
	if c == nil || c.client == nil {
		return nil
	}
	return c.client.Close()
}
