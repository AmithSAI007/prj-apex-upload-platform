package trace

import (
	"context"
	"testing"

	"github.com/AmithSAI007/prj-apex-upload-platform/pkg/constants"
)

func TestGenerateTraceID_Length(t *testing.T) {
	id := GenerateTraceID()
	if len(id) != 16 {
		t.Fatalf("expected trace ID length 16, got %d", len(id))
	}
}

func TestGenerateTraceID_Unique(t *testing.T) {
	a := GenerateTraceID()
	b := GenerateTraceID()
	if a == b {
		t.Fatalf("expected unique trace IDs, got %s twice", a)
	}
}

func TestContextWithTraceID_RoundTrip(t *testing.T) {
	ctx := context.Background()
	ctx = ContextWithTraceID(ctx, "test-trace-id")
	got := TraceIDFromContext(ctx)
	if got != "test-trace-id" {
		t.Fatalf("expected test-trace-id, got %s", got)
	}
}

func TestTraceIDFromContext_NilContext(t *testing.T) {
	got := TraceIDFromContext(context.TODO())
	if got != "" {
		t.Fatalf("expected empty string for nil context, got %s", got)
	}
}

func TestTraceIDFromContext_MissingKey(t *testing.T) {
	got := TraceIDFromContext(context.Background())
	if got != "" {
		t.Fatalf("expected empty string for missing key, got %s", got)
	}
}

func TestTraceIDFromContext_WrongType(t *testing.T) {
	ctx := context.WithValue(context.Background(), constants.CtxTraceIDKey, 12345)
	got := TraceIDFromContext(ctx)
	if got != "" {
		t.Fatalf("expected empty string for non-string value, got %s", got)
	}
}

func TestDataFromContext_NilContext(t *testing.T) {
	got := DataFromContext(context.TODO(), "any_key")
	if got != "" {
		t.Fatalf("expected empty string for nil context, got %s", got)
	}
}

func TestDataFromContext_MissingKey(t *testing.T) {
	got := DataFromContext(context.Background(), "missing")
	if got != "" {
		t.Fatalf("expected empty string for missing key, got %s", got)
	}
}

func TestDataFromContext_StringValue(t *testing.T) {
	ctx := context.WithValue(context.Background(), constants.CtxUserIDKey, "u123")
	got := DataFromContext(ctx, constants.CtxUserIDKey)
	if got != "u123" {
		t.Fatalf("expected u123, got %s", got)
	}
}

func TestDataFromContext_NonStringValue(t *testing.T) {
	ctx := context.WithValue(context.Background(), constants.CtxUserIDKey, 42)
	got := DataFromContext(ctx, constants.CtxUserIDKey)
	if got != "" {
		t.Fatalf("expected empty string for non-string value, got %s", got)
	}
}
