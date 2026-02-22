package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/AmithSAI007/prj-apex-upload-platform/api/dto"
	"github.com/AmithSAI007/prj-apex-upload-platform/internal/service"
	"github.com/AmithSAI007/prj-apex-upload-platform/pkg/trace"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type AuthMiddleware struct {
	logger       *zap.Logger
	tokenService service.TokenInterface
}

type ctxKey string

const (
	authHeaderKey        = "Authorization"
	bearerPrefix         = "Bearer"
	ctxUserIDKey  ctxKey = "user_id"
)

func NewAuthMiddleware(logger *zap.Logger, tokenService service.TokenInterface) *AuthMiddleware {
	return &AuthMiddleware{
		logger:       logger,
		tokenService: tokenService,
	}

}

func (m *AuthMiddleware) Authenticate() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		authHeader := ctx.GetHeader(authHeaderKey)
		if authHeader == "" {
			m.logger.Warn("Missing Authorization header", zap.String("trace_id", trace.TraceIDFromContext(ctx)))
			abortUnauthorized(ctx, "Missing Authorization header")
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], bearerPrefix) {
			m.logger.Warn("Invalid Authorization header format", zap.String("trace_id", trace.TraceIDFromContext(ctx)))
			abortUnauthorized(ctx, "Invalid Authorization header format")
			return
		}

		tokenStr := parts[1]

		claims, err := m.tokenService.ValidateToken(tokenStr)
		if err != nil {
			switch {
			case errors.Is(err, service.ErrExpiredToken):
				m.logger.Warn("Token expired", zap.String("trace_id", trace.TraceIDFromContext(ctx)))
				abortUnauthorized(ctx, "Token expired")
			case errors.Is(err, service.ErrInvalidSignature):
				m.logger.Warn("Invalid token signature", zap.String("trace_id", trace.TraceIDFromContext(ctx)))
				abortUnauthorized(ctx, "Invalid token signature")
			case errors.Is(err, service.ErrInvalidTokenType):
				m.logger.Warn("Invalid token type", zap.String("trace_id", trace.TraceIDFromContext(ctx)))
				abortUnauthorized(ctx, "Invalid token type")
			default:
				m.logger.Warn("Token validation failed", zap.Error(err), zap.String("trace_id", trace.TraceIDFromContext(ctx)))
				abortUnauthorized(ctx, "Invalid token")

			}
		}

		c := context.WithValue(ctx.Request.Context(), ctxUserIDKey, claims.UserID)
		ctx.Request = ctx.Request.WithContext(c)
		ctx.Next()
	}
}

func abortUnauthorized(ctx *gin.Context, message string) {
	ctx.AbortWithStatusJSON(http.StatusUnauthorized, dto.ErrorResponse{
		Error: dto.ErrorPayload{
			Code:      dto.ErrorCodeUnauthorized,
			Message:   message,
			RequestID: trace.TraceIDFromContext(ctx),
		},
	})
}
