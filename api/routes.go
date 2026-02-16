package api

import (
	"github.com/AmithSAI007/prj-apex-upload-platform/api/handler"
	"github.com/AmithSAI007/prj-apex-upload-platform/docs"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/swaggo/files"
	"github.com/swaggo/gin-swagger"
)

type HandlerRegistry struct {
	Upload *handler.UploadHandler
}

func SetupRoutes(router *gin.Engine, handlers *HandlerRegistry) {
	v1 := router.Group("/api/v1")

	uploadGroup := v1.Group("/uploads")
	uploadGroup.POST("", handlers.Upload.Create)
	uploadGroup.POST("/:uploadId/resume", handlers.Upload.Resume)
	uploadGroup.GET("/:uploadId", handlers.Upload.GetStatus)
	uploadGroup.POST("/:uploadId/status", handlers.Upload.QueryStatus)
	uploadGroup.POST("/:uploadId/cancel", handlers.Upload.Cancel)

	v1.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, ginSwagger.URL("/docs/doc.json")))

	router.GET("/docs/doc.json", func(ctx *gin.Context) {
		ctx.Writer.Header().Set("Content-Type", "application/json")
		ctx.Writer.WriteHeader(200)
		ctx.Writer.Write([]byte(docs.SwaggerInfo.ReadDoc()))
	})

	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

}
