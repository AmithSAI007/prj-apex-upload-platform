package storage

import (
	"context"
	"errors"
	"testing"
	"time"

	internalerrors "github.com/AmithSAI007/prj-apex-upload-platform/internal/errors"
	"github.com/sony/gobreaker/v2"
)

// cbTestSignedURLClient is a minimal fake for SignedURLClient used in CB tests.
type cbTestSignedURLClient struct {
	url string
	err error
}

func (f *cbTestSignedURLClient) SignResumableUploadURL(_ context.Context, _, _, _ string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.url, nil
}

// gcsTripImmediately returns gobreaker.Settings that trip after 1 consecutive failure.
func gcsTripImmediately() gobreaker.Settings {
	return gobreaker.Settings{
		Name: "test-gcs-cb",
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 1
		},
	}
}

func TestCBSignedURLClient_Interface(t *testing.T) {
	var _ SignedURLClient = (*CBSignedURLClient)(nil)
}

func TestCBSignedURLClient_PassThrough(t *testing.T) {
	inner := &cbTestSignedURLClient{url: "https://storage.googleapis.com/signed"}
	cb := NewCBSignedURLClient(inner, gcsTripImmediately())

	url, err := cb.SignResumableUploadURL(context.Background(), "bucket", "obj", "sa@test")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if url != "https://storage.googleapis.com/signed" {
		t.Fatalf("expected signed URL, got %q", url)
	}
}

func TestCBSignedURLClient_InnerError(t *testing.T) {
	inner := &cbTestSignedURLClient{err: errors.New("signing failed")}
	cb := NewCBSignedURLClient(inner, gcsTripImmediately())

	_, err := cb.SignResumableUploadURL(context.Background(), "bucket", "obj", "sa@test")
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "signing failed" {
		t.Fatalf("expected inner error, got %v", err)
	}
}

func TestCBSignedURLClient_CircuitOpen(t *testing.T) {
	inner := &cbTestSignedURLClient{err: errors.New("transient")}
	cb := NewCBSignedURLClient(inner, gcsTripImmediately())

	// First call fails and trips the breaker.
	_, _ = cb.SignResumableUploadURL(context.Background(), "bucket", "obj", "sa@test")

	// Second call should be rejected by the open circuit.
	_, err := cb.SignResumableUploadURL(context.Background(), "bucket", "obj", "sa@test")
	if err == nil {
		t.Fatal("expected circuit open error")
	}
	if !errors.Is(err, internalerrors.ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}
}

func TestCBSignedURLClient_Recovery(t *testing.T) {
	inner := &cbTestSignedURLClient{err: errors.New("fail")}

	settings := gobreaker.Settings{
		Name:        "test-gcs-recovery",
		MaxRequests: 1,
		Timeout:     1 * time.Millisecond,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 1
		},
	}
	cb := NewCBSignedURLClient(inner, settings)

	// Trip the breaker.
	_, _ = cb.SignResumableUploadURL(context.Background(), "bucket", "obj", "sa@test")

	// Fix the inner client.
	inner.err = nil
	inner.url = "https://recovered.example.com"

	// Wait for the breaker timeout to elapse.
	time.Sleep(5 * time.Millisecond)

	url, err := cb.SignResumableUploadURL(context.Background(), "bucket", "obj", "sa@test")
	if err != nil {
		t.Fatalf("expected recovery, got %v", err)
	}
	if url != "https://recovered.example.com" {
		t.Fatalf("expected recovered URL, got %q", url)
	}
}
