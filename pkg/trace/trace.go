package trace

import (
	"context"
	"crypto/rand"
	"encoding/hex"
)

type traceContextKey string

const traceIDContextKey traceContextKey = "trace_id"

// GenerateTraceID returns a cryptographically-random trace id.
func GenerateTraceID() string {
	buffer := make([]byte, 8)
	if _, err := rand.Read(buffer); err != nil {
		return "0000000000000000"
	}
	return hex.EncodeToString(buffer)
}

// ContextWithTraceID stores the trace id in the provided context and returns a new context.
func ContextWithTraceID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, traceIDContextKey, id)
}

// TraceIDFromContext returns the trace ID stored in a context.
func TraceIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if value := ctx.Value(traceIDContextKey); value != nil {
		if id, ok := value.(string); ok {
			return id
		}
	}
	return ""
}
