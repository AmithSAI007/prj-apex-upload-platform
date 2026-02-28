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

type AuthMiddleware struct {
	logger       *zap.Logger
	tokenService service.TokenInterface
	trace        trace.Tracer
}

const (
	authHeaderKey = "Authorization"
	bearerPrefix  = "Bearer"
)

func NewAuthMiddleware(logger *zap.Logger, tokenService service.TokenInterface, tracer trace.Tracer) *AuthMiddleware {
	return &AuthMiddleware{
		logger:       logger,
		tokenService: tokenService,
		trace:        tracer,
	}

}

func (m *AuthMiddleware) Authenticate() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		authHeader := ctx.GetHeader(authHeaderKey)
		traceID := pkgtrace.TraceIDFromContext(ctx)
		_, span := m.trace.Start(ctx.Request.Context(), "/api/middleware/auth/Authenticate")
		defer span.End()

		if authHeader == "" {
			span.RecordError(errors.New("missing Authorization header"))
			span.SetStatus(codes.Error, "Missing Authorization header")
			m.logger.Warn("Missing Authorization header", zap.String("trace_id", traceID))
			abortUnauthorized(ctx, "Missing Authorization header", traceID)
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], bearerPrefix) {
			span.RecordError(errors.New("invalid Authorization header format"))
			span.SetStatus(codes.Error, "Invalid Authorization header format")
			m.logger.Warn("Invalid Authorization header format", zap.String("trace_id", traceID))
			abortUnauthorized(ctx, "Invalid Authorization header format", traceID)
			return
		}

		tokenStr := parts[1]

		claims, err := m.tokenService.ValidateToken(tokenStr)
		if err != nil {
			switch {
			case errors.Is(err, service.ErrExpiredToken):
				m.logger.Warn("Token expired", zap.String("trace_id", traceID))
				span.RecordError(err)
				span.SetStatus(codes.Error, "Token expired")
				abortUnauthorized(ctx, "Token expired", traceID)
			case errors.Is(err, service.ErrInvalidSignature):
				span.RecordError(err)
				span.SetStatus(codes.Error, "Invalid token signature")
				m.logger.Warn("Invalid token signature", zap.String("trace_id", traceID))
				abortUnauthorized(ctx, "Invalid token signature", traceID)
			case errors.Is(err, service.ErrInvalidTokenType):
				span.RecordError(err)
				span.SetStatus(codes.Error, "Invalid token type")
				m.logger.Warn("Invalid token type", zap.String("trace_id", traceID))
				abortUnauthorized(ctx, "Invalid token type", traceID)
			default:
				span.RecordError(err)
				span.SetStatus(codes.Error, "Invalid token")
				m.logger.Warn("Token validation failed", zap.Error(err), zap.String("trace_id", traceID))
				abortUnauthorized(ctx, "Invalid token", traceID)

			}
			return
		}

		span.SetAttributes(attribute.String("user_id", claims.UserID))
		span.SetStatus(codes.Ok, "Token validated successfully")
		c := context.WithValue(ctx.Request.Context(), constants.CtxUserIDKey, claims.UserID)
		ctx.Request = ctx.Request.WithContext(c)
		ctx.Next()
	}
}

func abortUnauthorized(ctx *gin.Context, message string, traceID string) {
	ctx.AbortWithStatusJSON(http.StatusUnauthorized, dto.ErrorResponse{
		Error: dto.ErrorPayload{
			Code:      dto.ErrorCodeUnauthorized,
			Message:   message,
			RequestID: traceID,
		},
	})
}
