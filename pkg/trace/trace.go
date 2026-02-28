package trace

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	"github.com/AmithSAI007/prj-apex-upload-platform/pkg/constants"
)

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
	return context.WithValue(ctx, constants.CtxTraceIDKey, id)
}

// TraceIDFromContext returns the trace ID stored in a context.
func TraceIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if value := ctx.Value(constants.CtxTraceIDKey); value != nil {
		if id, ok := value.(string); ok {
			return id
		}
	}
	return ""
}

func DataFromContext(ctx context.Context, key string) string {
	if ctx == nil {
		return ""
	}
	if value := ctx.Value(key); value != nil {
		if id, ok := value.(string); ok {
			return id
		}
	}
	return ""
}
