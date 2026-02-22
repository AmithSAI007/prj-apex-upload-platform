package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/AmithSAI007/prj-apex-upload-platform/api/dto"
	"github.com/AmithSAI007/prj-apex-upload-platform/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type AuthMiddleware struct {
	logger       *zap.Logger
	tokenService service.TokenInterface
}

const (
	AUTH_HEADER_KEY = "Authorization"
	BEARER_PREFIX   = "Bearer"
)

func NewAuthMiddleware(logger *zap.Logger, tokenService service.TokenInterface) *AuthMiddleware {
	return &AuthMiddleware{
		logger:       logger,
		tokenService: tokenService,
	}

}

func (m *AuthMiddleware) Authenticate() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		authHeader := ctx.GetHeader(AUTH_HEADER_KEY)
		if authHeader == "" {
			m.logger.Warn("Missing Authorization header", zap.String("trace_id", traceID(ctx)))
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, dto.ErrorResponse{
				Error: dto.ErrorPayload{
					Code:      dto.ErrorCodeUnauthorized,
					Message:   "Missing Authorization header",
					RequestID: traceID(ctx),
				},
			})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != BEARER_PREFIX {
			m.logger.Warn("Invalid Authorization header format", zap.String("trace_id", traceID(ctx)))
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, dto.ErrorResponse{
				Error: dto.ErrorPayload{
					Code:      dto.ErrorCodeUnauthorized,
					Message:   "Invalid Authorization header format",
					RequestID: traceID(ctx),
				},
			})
			return
		}

		tokenStr := parts[1]

		claims, err := m.tokenService.ValidateToken(tokenStr)
		if err != nil {
			m.logger.Warn("Token validation failed", zap.String("trace_id", traceID(ctx)), zap.Error(err))
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, dto.ErrorResponse{
				Error: dto.ErrorPayload{
					Code:      dto.ErrorCodeUnauthorized,
					Message:   "Invalid or expired token",
					RequestID: traceID(ctx),
				},
			})
			return
		}

		c := context.WithValue(ctx.Request.Context(), "user_id", claims.UserID)
		ctx.Request = ctx.Request.WithContext(c)
		ctx.Next()
	}
}
