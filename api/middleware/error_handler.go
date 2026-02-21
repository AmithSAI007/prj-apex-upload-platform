package middleware

import (
	"net/http"

	"github.com/AmithSAI007/prj-apex-upload-platform/api/dto"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func ErrorHandler(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {

		defer func() {
			if r := recover(); r != nil {
				logger.Error("Recovered from panic", zap.Any("error", r), zap.String("trace_id", traceID(c)))
				c.AbortWithStatusJSON(
					http.StatusInternalServerError,
					dto.ErrorResponse{
						Error: dto.ErrorPayload{
							Code:      dto.ErrorCodeInternal,
							Message:   "Internal Server Error",
							RequestID: traceID(c),
						},
					},
				)
			}
		}()

		c.Next()

		if len(c.Errors) > 0 {
			for _, e := range c.Errors {
				logger.Error("Request error", zap.Error(e.Err), zap.String("trace_id", traceID(c)))
			}
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{
				Error: dto.ErrorPayload{
					Code:      dto.ErrorCodeInvalidArgument,
					Message:   "Bad Request",
					RequestID: traceID(c),
				},
			})
		}

	}
}

func traceID(ctx *gin.Context) string {
	if value, ok := ctx.Get(TraceIDKey); ok {
		if id, ok := value.(string); ok {
			return id
		}
	}
	return ""
}
