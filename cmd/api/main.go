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
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/AmithSAI007/prj-apex-upload-platform/api"
	"github.com/AmithSAI007/prj-apex-upload-platform/api/handler"
	"github.com/AmithSAI007/prj-apex-upload-platform/api/middleware"
	"github.com/AmithSAI007/prj-apex-upload-platform/internal/config"
	"github.com/AmithSAI007/prj-apex-upload-platform/internal/secrets"
	"github.com/AmithSAI007/prj-apex-upload-platform/internal/service"
	"github.com/AmithSAI007/prj-apex-upload-platform/internal/storage"
	"github.com/AmithSAI007/prj-apex-upload-platform/internal/validation"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/sony/gobreaker/v2"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"
)

func main() {
	// Load application configuration from environment variables and .env files.
	cfg, err := config.LoadConfig(".")
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize the structured logger (JSON in production, console in development).
	logger, err := config.NewLogger()
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer func() {
		_ = logger.Sync()
	}()

	// Create a root context for the application lifecycle.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up the OpenTelemetry tracing pipeline (OTLP exporter, sampler, propagators).
	otelShutdown, err := config.InitTracer(cfg, ctx)
	if err != nil {
		logger.Fatal("Failed to initialize tracer", zap.Error(err))
	}
	defer func() {
		err = errors.Join(err, otelShutdown(ctx))
	}()

	// Create a named tracer for this application's spans.
	tracer := otel.Tracer("github.com/AmithSAI007/prj-apex-upload-platform")

	// Initialize the request validator for DTO struct validation.
	validate := validator.New()
	validationCtx := validation.NewValidationContext(cfg.AllowedContentTypes, cfg.MaxFileSizeBytes)
	if err := validationCtx.RegisterValidators(validate); err != nil {
		logger.Fatal("Failed to register custom validators", zap.Error(err))
	}
	if cfg.GCPProjectID == "" {
		logger.Fatal("Missing GCP project ID")
	}

	// Initialize the GCS client for generating signed upload URLs.
	gcsClient, err := storage.NewGCSClient(ctx, tracer, cfg)
	if err != nil {
		logger.Fatal("Failed to initialize GCS client", zap.Error(err))
	}
	defer func() {
		_ = gcsClient.Close()
	}()

	// Initialize the Firestore client for session persistence.
	firestoreClient, err := storage.NewFirestoreClient(ctx, cfg.GCPProjectID, cfg.FirestoreDatabaseID)
	if err != nil {
		logger.Fatal("Failed to initialize Firestore client", zap.Error(err))
	}
	defer func() {
		_ = firestoreClient.Close()
	}()

	// Initialize the Secret Manager client for loading JWT public keys.
	secretClient, err := secrets.NewSecretsClient(ctx, logger)
	if err != nil {
		logger.Fatal("Failed to initialize Secret Manager client", zap.Error(err))
	}
	defer func() {
		_ = secretClient.Close()
	}()

	// Wire up the service layer: session store, key service, token service, upload service.
	firestoreStore := service.NewFirestoreUploadSessionStore(firestoreClient.Client(), logger, tracer, cfg)
	keyService := service.NewSMKeyService(logger, secretClient)
	publicKey, err := keyService.LoadKey(ctx, cfg.JWT_PUBLIC_KEY_PATH)
	if err != nil {
		logger.Fatal("Failed to load JWT keys", zap.Error(err))
	}
	tokenService := service.NewTokenService(logger, cfg, publicKey)

	// Configure circuit breaker settings shared by both decorators.
	cbOnStateChange := func(name string, from gobreaker.State, to gobreaker.State) {
		logger.Warn("circuit breaker state change",
			zap.String("name", name),
			zap.String("from", from.String()),
			zap.String("to", to.String()),
		)
	}

	storeCBSettings := gobreaker.Settings{
		Name:        "firestore-upload-session-store",
		MaxRequests: cfg.CBMaxRequests,
		Interval:    time.Duration(cfg.CBIntervalSeconds) * time.Second,
		Timeout:     time.Duration(cfg.CBTimeoutSeconds) * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= cfg.CBConsecutiveFailures
		},
		OnStateChange: cbOnStateChange,
	}

	gcsCBSettings := gobreaker.Settings{
		Name:        "gcs-signed-url-client",
		MaxRequests: cfg.CBMaxRequests,
		Interval:    time.Duration(cfg.CBIntervalSeconds) * time.Second,
		Timeout:     time.Duration(cfg.CBTimeoutSeconds) * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= cfg.CBConsecutiveFailures
		},
		OnStateChange: cbOnStateChange,
	}

	// Wrap the raw implementations with circuit breakers.
	// Call chain: Service → CB Decorator → Retry (inside real impl) → Firestore/GCS.
	store := service.NewCBUploadSessionStore(firestoreStore, storeCBSettings)
	cbGCSClient := storage.NewCBSignedURLClient(gcsClient, gcsCBSettings)

	uploadService := service.NewUploadService(logger, cbGCSClient, cfg, store, tracer)
	authMiddleware := middleware.NewAuthMiddleware(logger, tokenService, tracer)

	// Initialize the per-client rate limiter for authenticated endpoints.
	perClientLimiter := middleware.NewPerClientRateLimiter(
		logger,
		cfg.PerClientRateLimitRPS,
		cfg.PerClientRateLimitBurst,
		30*time.Minute,
		cfg.MaxRateLimitClients,
	)
	defer perClientLimiter.Stop()

	// Create the HTTP handler with the upload service.
	uploadHandler := handler.NewUploadHandler(logger, validate, uploadService)

	// Configure the Gin router with middleware stack.
	router := gin.New()
	router.MaxMultipartMemory = 32 << 20                                                    // 32 MiB
	router.Use(gin.Recovery())                                                              // Recover from panics and return 500.
	router.Use(middleware.RequestContext())                                                 // Inject trace/request IDs.
	router.Use(middleware.GlobalRateLimiter(cfg.RateLimitRPS, cfg.RateLimitBurst))          // Enforce global request rate limit.
	router.Use(middleware.ErrorHandler(logger))                                             // Log unhandled errors.
	router.Use(middleware.PrometheusMetrics())                                              // Record request metrics.
	router.Use(middleware.CORSMiddleware(cfg.CorsAllowedOrigins, cfg.CorsAllowCredentials)) // Set CORS headers.
	router.Use(otelgin.Middleware(cfg.OTEL_SERVICE_NAME))                                   // OTel HTTP instrumentation.

	// Register all API routes.
	handlers := &api.HandlerRegistry{
		Upload:           uploadHandler,
		Auth:             authMiddleware,
		PerClientLimiter: perClientLimiter,
	}

	api.SetupRoutes(router, handlers)

	svr := &http.Server{
		Addr:              cfg.HttpPort,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("Starting server", zap.String("port", cfg.HttpPort))
		if err := svr.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal("Server failed", zap.Error(err))
		}
	}()

	<-ctx.Done()
	logger.Info("Shutting down server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := svr.Shutdown(shutdownCtx); err != nil {
		logger.Fatal("Server forced to shutdown", zap.Error(err))
	}

	logger.Info("Server exiting")

}
