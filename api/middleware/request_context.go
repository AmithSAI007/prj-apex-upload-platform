package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	RequestIDHeader = "X-Request-ID"
	TraceIDKey      = "trace_id"
)

type traceContextKey string

const traceIDContextKey traceContextKey = "trace_id"

// RequestContext injects a trace ID into the request context and response header.
func RequestContext() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		requestID := strings.TrimSpace(ctx.GetHeader(RequestIDHeader))
		if requestID == "" {
			requestID = "req_" + generateTraceID()
		}
		ctx.Set(TraceIDKey, requestID)
		ctx.Request = ctx.Request.WithContext(context.WithValue(ctx.Request.Context(), traceIDContextKey, requestID))
		ctx.Writer.Header().Set(RequestIDHeader, requestID)
		ctx.Next()
	}
}

func generateTraceID() string {
	buffer := make([]byte, 8)
	if _, err := rand.Read(buffer); err != nil {
		return "0000000000000000"
	}
	return hex.EncodeToString(buffer)
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
