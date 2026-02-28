// @title Apex Upload Platform API
// @version 1.0
// @description Enterprise-grade resumable upload API for GCS.
// @termsOfService https://example.com/terms
// @contact.name Platform API Support
// @contact.url https://example.com/support
// @contact.email support@example.com
// @license.name Proprietary
// @license.url https://example.com/license
// @BasePath /api/v1
// @schemes https http
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
package main

import (
	"context"
	"errors"
	"log"

	"github.com/AmithSAI007/prj-apex-upload-platform/api"
	"github.com/AmithSAI007/prj-apex-upload-platform/api/handler"
	"github.com/AmithSAI007/prj-apex-upload-platform/api/middleware"
	"github.com/AmithSAI007/prj-apex-upload-platform/internal/config"
	"github.com/AmithSAI007/prj-apex-upload-platform/internal/secrets"
	"github.com/AmithSAI007/prj-apex-upload-platform/internal/service"
	"github.com/AmithSAI007/prj-apex-upload-platform/internal/storage"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"
)

func main() {

	cfg, err := config.LoadConfig(".")
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	logger, err := config.NewLogger()
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Sync()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	otelShutdown, err := config.InitTracer(cfg, ctx)
	if err != nil {
		logger.Fatal("Failed to initialize tracer", zap.Error(err))
	}
	defer func() {
		err = errors.Join(err, otelShutdown(ctx))
	}()

	tracer := otel.Tracer("github.com/AmithSAI007/prj-apex-upload-platform")

	validate := validator.New()
	if cfg.GCPProjectID == "" {
		logger.Fatal("Missing GCP project ID")
	}
	gcsClient, err := storage.NewGCSClient(ctx, tracer)
	if err != nil {
		logger.Fatal("Failed to initialize GCS client", zap.Error(err))
	}
	defer gcsClient.Close()

	firestoreClient, err := storage.NewFirestoreClient(ctx, cfg.GCPProjectID, cfg.FirestoreDatabaseID)
	if err != nil {
		logger.Fatal("Failed to initialize Firestore client", zap.Error(err))
	}
	defer firestoreClient.Close()

	secretClient, err := secrets.NewSecretsClient(ctx, logger)
	if err != nil {
		logger.Fatal("Failed to initialize Secret Manager client", zap.Error(err))
	}
	defer secretClient.Close()

	store := service.NewFirestoreUploadSessionStore(firestoreClient.Client(), logger, tracer)
	keyService := service.NewSMKeyService(logger, secretClient)
	publicKey, err := keyService.LoadKey(ctx, cfg.JWT_PUBLIC_KEY_PATH)
	if err != nil {
		logger.Fatal("Failed to load JWT keys", zap.Error(err))
	}
	tokenService := service.NewTokenService(logger, cfg, publicKey)
	uploadService := service.NewUploadService(logger, gcsClient, cfg, store, tracer)
	authMiddleware := middleware.NewAuthMiddleware(logger, tokenService, tracer)

	uploadHandler := handler.NewUploadHandler(logger, validate, uploadService)

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(middleware.RequestContext())
	router.Use(middleware.ErrorHandler(logger))
	router.Use(middleware.PrometheusMetrics())
	router.Use(otelgin.Middleware(cfg.OTEL_SERVICE_NAME))

	handlers := &api.HandlerRegistry{
		Upload: uploadHandler,
		Auth:   authMiddleware,
	}

	api.SetupRoutes(router, handlers)

	logger.Info("Server starting on port 8080...")
	if err := router.Run(":8080"); err != nil {
		logger.Error(err.Error())
	}

}
