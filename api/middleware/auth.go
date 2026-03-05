// Package middleware provides Gin middleware for cross-cutting concerns
// including authentication, request context propagation, error handling,
// and Prometheus metrics collection.
package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/AmithSAI007/prj-apex-upload-platform/api/dto"
	"github.com/AmithSAI007/prj-apex-upload-platform/internal/service"
	"github.com/AmithSAI007/prj-apex-upload-platform/pkg/constants"

	pkgtrace "github.com/AmithSAI007/prj-apex-upload-platform/pkg/trace"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// AuthMiddleware validates JWT bearer tokens on incoming requests and injects
// the authenticated user's identity into the request context. It creates an
// OpenTelemetry span for each authentication attempt with detailed events.
type AuthMiddleware struct {
	logger       *zap.Logger
	tokenService service.TokenInterface
	trace        trace.Tracer
}

const (
	// authHeaderKey is the HTTP header name for the Authorization header.
	authHeaderKey = "Authorization"
	// bearerPrefix is the expected prefix for Bearer token authentication.
	bearerPrefix = "Bearer"
)

// NewAuthMiddleware constructs an AuthMiddleware with the given logger,
// token validation service, and OpenTelemetry tracer.
func NewAuthMiddleware(logger *zap.Logger, tokenService service.TokenInterface, tracer trace.Tracer) *AuthMiddleware {
	return &AuthMiddleware{
		logger:       logger,
		tokenService: tokenService,
		trace:        tracer,
	}

}

// Authenticate returns a Gin middleware handler that extracts and validates
// JWT bearer tokens. On success, it stores the user ID in the request context
// and calls the next handler. On failure, it aborts with HTTP 401.
func (m *AuthMiddleware) Authenticate() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		authHeader := ctx.GetHeader(authHeaderKey)
		traceID := pkgtrace.TraceIDFromContext(ctx)
		_, span := m.trace.Start(ctx.Request.Context(), "/api/middleware/auth/Authenticate")
		defer span.End()

		// Record an event marking the start of authentication.
		span.AddEvent("auth.start", trace.WithAttributes(
			attribute.String("trace_id", traceID),
		))

		if authHeader == "" {
			span.RecordError(errors.New("missing Authorization header"))
			span.SetStatus(codes.Error, "Missing Authorization header")
			span.AddEvent("auth.failed", trace.WithAttributes(
				attribute.String("reason", "missing_authorization_header"),
			))
			m.logger.Warn("Missing Authorization header", zap.String("trace_id", traceID))
			abortUnauthorized(ctx, "Missing Authorization header", traceID)
			return
		}

		// Parse the "Bearer <token>" format.
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], bearerPrefix) {
			span.RecordError(errors.New("invalid Authorization header format"))
			span.SetStatus(codes.Error, "Invalid Authorization header format")
			span.AddEvent("auth.failed", trace.WithAttributes(
				attribute.String("reason", "invalid_header_format"),
			))
			m.logger.Warn("Invalid Authorization header format", zap.String("trace_id", traceID))
			abortUnauthorized(ctx, "Invalid Authorization header format", traceID)
			return
		}

		tokenStr := parts[1]

		// Validate the JWT and extract claims.
		claims, err := m.tokenService.ValidateToken(tokenStr)
		if err != nil {
			switch {
			case errors.Is(err, service.ErrExpiredToken):
				m.logger.Warn("Token expired", zap.String("trace_id", traceID))
				span.RecordError(err)
				span.SetStatus(codes.Error, "Token expired")
				span.AddEvent("auth.failed", trace.WithAttributes(
					attribute.String("reason", "token_expired"),
				))
				abortUnauthorized(ctx, "Token expired", traceID)
			case errors.Is(err, service.ErrInvalidSignature):
				span.RecordError(err)
				span.SetStatus(codes.Error, "Invalid token signature")
				span.AddEvent("auth.failed", trace.WithAttributes(
					attribute.String("reason", "invalid_signature"),
				))
				m.logger.Warn("Invalid token signature", zap.String("trace_id", traceID))
				abortUnauthorized(ctx, "Invalid token signature", traceID)
			case errors.Is(err, service.ErrInvalidTokenType):
				span.RecordError(err)
				span.SetStatus(codes.Error, "Invalid token type")
				span.AddEvent("auth.failed", trace.WithAttributes(
					attribute.String("reason", "invalid_token_type"),
				))
				m.logger.Warn("Invalid token type", zap.String("trace_id", traceID))
				abortUnauthorized(ctx, "Invalid token type", traceID)
			default:
				span.RecordError(err)
				span.SetStatus(codes.Error, "Invalid token")
				span.AddEvent("auth.failed", trace.WithAttributes(
					attribute.String("reason", "invalid_token"),
				))
				m.logger.Warn("Token validation failed", zap.Error(err), zap.String("trace_id", traceID))
				abortUnauthorized(ctx, "Invalid token", traceID)

			}
			return
		}

		// Authentication succeeded: record user identity and propagate downstream.
		span.SetAttributes(attribute.String("user_id", claims.UserID))
		span.SetStatus(codes.Ok, "Token validated successfully")
		span.AddEvent("auth.success", trace.WithAttributes(
			attribute.String("user_id", claims.UserID),
		))
		c := context.WithValue(ctx.Request.Context(), constants.CtxUserIDKey, claims.UserID)
		c = context.WithValue(c, constants.CtxTraceIDKey, traceID)
		ctx.Request = ctx.Request.WithContext(c)
		ctx.Next()
	}
}

// abortUnauthorized sends a standardized 401 JSON error response and aborts
// the Gin handler chain.
func abortUnauthorized(ctx *gin.Context, message string, traceID string) {
	ctx.AbortWithStatusJSON(http.StatusUnauthorized, dto.ErrorResponse{
		Error: dto.ErrorPayload{
			Code:      dto.ErrorCodeUnauthorized,
			Message:   message,
			RequestID: traceID,
		},
	})
}
