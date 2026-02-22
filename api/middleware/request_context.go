package middleware

import (
	"github.com/AmithSAI007/prj-apex-upload-platform/pkg/trace"
	"github.com/gin-gonic/gin"
	"strings"
)

const (
	RequestIDHeader = "X-Request-ID"
	TraceIDKey      = "trace_id"
)

// RequestContext injects a trace ID into the request context and response header.
func RequestContext() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		requestID := strings.TrimSpace(ctx.GetHeader(RequestIDHeader))
		if requestID == "" {
			requestID = "req_" + trace.GenerateTraceID()
		}
		ctx.Set(TraceIDKey, requestID)
		ctx.Request = ctx.Request.WithContext(trace.ContextWithTraceID(ctx.Request.Context(), requestID))
		ctx.Writer.Header().Set(RequestIDHeader, requestID)
		ctx.Next()
	}
}
