package storage

import (
	"context"
	"strings"
	"testing"

	"cloud.google.com/go/storage"
	"go.opentelemetry.io/otel"
	"google.golang.org/api/option"
)

func TestSignedURLClientInterface(t *testing.T) {
	var _ SignedURLClient = (*GCSClient)(nil)
}

// TestGCSClient_Close_NilReceiver verifies that Close on a nil *GCSClient
// returns nil without panicking.
func TestGCSClient_Close_NilReceiver(t *testing.T) {
	var c *GCSClient
	if err := c.Close(); err != nil {
		t.Fatalf("expected nil error for nil receiver, got %v", err)
	}
}

// TestGCSClient_Close_NilInnerClient verifies that Close returns nil when the
// inner storage client is nil (e.g., partially initialized struct).
func TestGCSClient_Close_NilInnerClient(t *testing.T) {
	c := &GCSClient{client: nil, trace: otel.Tracer("test")}
	if err := c.Close(); err != nil {
		t.Fatalf("expected nil error for nil inner client, got %v", err)
	}
}

// TestGCSClient_Client_Getter verifies the Client() accessor returns the
// underlying storage.Client (nil in this case since we cannot construct one
// without real GCP).
func TestGCSClient_Client_Getter(t *testing.T) {
	c := &GCSClient{client: nil, trace: otel.Tracer("test")}
	if c.Client() != nil {
		t.Fatal("expected nil inner client")
	}
}

// TestSignResumableUploadURL_EmptyBucket verifies that validation rejects an
// empty bucket parameter.
func TestSignResumableUploadURL_EmptyBucket(t *testing.T) {
	c := &GCSClient{client: nil, trace: otel.Tracer("test")}
	_, err := c.SignResumableUploadURL(context.Background(), "", "object", "sa@test")
	if err == nil {
		t.Fatal("expected error for empty bucket")
	}
}

// TestSignResumableUploadURL_EmptyObjectName verifies that validation rejects
// an empty objectName parameter.
func TestSignResumableUploadURL_EmptyObjectName(t *testing.T) {
	c := &GCSClient{client: nil, trace: otel.Tracer("test")}
	_, err := c.SignResumableUploadURL(context.Background(), "bucket", "", "sa@test")
	if err == nil {
		t.Fatal("expected error for empty object name")
	}
}

// TestSignResumableUploadURL_EmptyServiceAccount verifies that validation
// rejects an empty serviceAccount parameter.
func TestSignResumableUploadURL_EmptyServiceAccount(t *testing.T) {
	c := &GCSClient{client: nil, trace: otel.Tracer("test")}
	_, err := c.SignResumableUploadURL(context.Background(), "bucket", "object", "")
	if err == nil {
		t.Fatal("expected error for empty service account")
	}
}

// TestSignResumableUploadURL_AllEmpty verifies that validation rejects when
// all three required parameters are empty.
func TestSignResumableUploadURL_AllEmpty(t *testing.T) {
	c := &GCSClient{client: nil, trace: otel.Tracer("test")}
	_, err := c.SignResumableUploadURL(context.Background(), "", "", "")
	if err == nil {
		t.Fatal("expected error for all empty params")
	}
}

// TestSignResumableUploadURL_SigningFailure creates a real (unauthenticated)
// GCS client and verifies the signing error path when credentials are missing.
func TestSignResumableUploadURL_SigningFailure(t *testing.T) {
	ctx := context.Background()
	client, err := storage.NewClient(ctx, option.WithoutAuthentication())
	if err != nil {
		t.Fatalf("failed to create unauthenticated GCS client: %v", err)
	}
	defer client.Close()

	c := &GCSClient{client: client, trace: otel.Tracer("test")}
	_, err = c.SignResumableUploadURL(ctx, "my-bucket", "my-object", "sa@example.com")
	if err == nil {
		t.Fatal("expected signing error with unauthenticated client")
	}
	if !strings.Contains(err.Error(), "SignedURL") {
		t.Fatalf("expected error about SignedURL, got: %v", err)
	}
}

// TestGCSClient_Close_RealClient verifies that Close works on a real (but
// unauthenticated) GCS client without error.
func TestGCSClient_Close_RealClient(t *testing.T) {
	ctx := context.Background()
	client, err := storage.NewClient(ctx, option.WithoutAuthentication())
	if err != nil {
		t.Fatalf("failed to create unauthenticated GCS client: %v", err)
	}
	c := &GCSClient{client: client, trace: otel.Tracer("test")}
	if err := c.Close(); err != nil {
		t.Fatalf("expected no error closing real client, got %v", err)
	}
}

// TestGCSClient_Client_RealClient verifies the Client() accessor returns
// the underlying non-nil storage.Client.
func TestGCSClient_Client_RealClient(t *testing.T) {
	ctx := context.Background()
	client, err := storage.NewClient(ctx, option.WithoutAuthentication())
	if err != nil {
		t.Fatalf("failed to create unauthenticated GCS client: %v", err)
	}
	defer client.Close()
	c := &GCSClient{client: client, trace: otel.Tracer("test")}
	if c.Client() == nil {
		t.Fatal("expected non-nil inner client")
	}
}
