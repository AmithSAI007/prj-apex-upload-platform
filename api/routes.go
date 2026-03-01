// Package api defines the HTTP route configuration and handler registry
// for the Apex Upload Platform API.
package api

import (
	"github.com/AmithSAI007/prj-apex-upload-platform/api/handler"
	"github.com/AmithSAI007/prj-apex-upload-platform/api/middleware"
	"github.com/AmithSAI007/prj-apex-upload-platform/docs"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/swaggo/files"
	"github.com/swaggo/gin-swagger"
)

// HandlerRegistry holds the handler and middleware instances needed for route
// registration. It acts as the dependency injection point for the HTTP layer.
type HandlerRegistry struct {
	Upload *handler.UploadHandler
	Auth   *middleware.AuthMiddleware
}

// SetupRoutes registers all API endpoints on the given Gin router.
//
// Protected routes (under /api/v1/uploads) require JWT authentication via the
// AuthMiddleware. Public routes include Swagger UI, the raw Swagger JSON spec,
// and Prometheus metrics.
func SetupRoutes(router *gin.Engine, handlers *HandlerRegistry) {
	v1 := router.Group("/api/v1")

	// Upload session endpoints: all require authenticated JWT bearer token.
	uploadGroup := v1.Group("/uploads")
	uploadGroup.Use(handlers.Auth.Authenticate())
	{
		uploadGroup.POST("", handlers.Upload.Create)
		uploadGroup.POST("/:uploadId/resume", handlers.Upload.Resume)
		uploadGroup.GET("/:uploadId", handlers.Upload.GetStatus)
		uploadGroup.POST("/:uploadId/status", handlers.Upload.QueryStatus)
		uploadGroup.POST("/:uploadId/cancel", handlers.Upload.Cancel)
	}

	// Swagger UI served at /api/v1/swagger/index.html.
	v1.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, ginSwagger.URL("/docs/doc.json")))

	// Raw OpenAPI JSON spec for programmatic consumers.
	router.GET("/docs/doc.json", func(ctx *gin.Context) {
		ctx.Writer.Header().Set("Content-Type", "application/json")
		ctx.Writer.WriteHeader(200)
		ctx.Writer.Write([]byte(docs.SwaggerInfo.ReadDoc()))
	})

	// Prometheus metrics endpoint.
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

}
