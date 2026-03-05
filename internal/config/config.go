// Package config provides application configuration loading, logger setup,
// and OpenTelemetry tracer initialization for the Apex Upload Platform.
package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config holds all application configuration values loaded from config.yaml
// and/or environment variables. Environment variables take precedence over
// file-based values when both are present.
type Config struct {
	// AppEnv is the deployment environment ("development", "production", etc.).
	AppEnv string `mapstructure:"APP_ENV"`
	// HttpPort is the TCP port the HTTP server listens on (default "8080").
	HttpPort string `mapstructure:"HTTP_PORT"`
	// GCPProjectID is the Google Cloud project ID used for GCS, Firestore, and Secret Manager.
	GCPProjectID string `mapstructure:"GCP_PROJECT_ID"`
	// FirestoreDatabaseID is the named Firestore database to connect to.
	FirestoreDatabaseID string `mapstructure:"FIRESTORE_DATABASE_ID"`
	// GCSBucket is the GCS bucket where uploaded objects are stored.
	GCSBucket string `mapstructure:"GCS_BUCKET"`
	// SERVICE_ACCOUNT_EMAIL is the IAM service account used for signing GCS URLs.
	SERVICE_ACCOUNT_EMAIL string `mapstructure:"SERVICE_ACCOUNT_EMAIL"`
	// JWT_PUBLIC_KEY_PATH is the Secret Manager resource path for the JWT public key.
	JWT_PUBLIC_KEY_PATH string `mapstructure:"JWT_PUBLIC_KEY_PATH"`
	// OTEL_SERVICE_NAME is the logical service name reported to the OpenTelemetry collector.
	OTEL_SERVICE_NAME string `mapstructure:"OTEL_SERVICE_NAME"`
	// RateLimitRPS is the global maximum requests per second allowed across all clients.
	RateLimitRPS float64 `mapstructure:"RATE_LIMIT_RPS"`
	// RateLimitBurst is the maximum burst size for the global rate limiter.
	RateLimitBurst int `mapstructure:"RATE_LIMIT_BURST"`
	// PerClientRateLimitRPS is the maximum requests per second allowed per authenticated user.
	PerClientRateLimitRPS float64 `mapstructure:"PER_CLIENT_RATE_LIMIT_RPS"`
	// PerClientRateLimitBurst is the maximum burst size for per-client rate limiting.
	PerClientRateLimitBurst int `mapstructure:"PER_CLIENT_RATE_LIMIT_BURST"`
	// MaxRateLimitClients is the maximum number of unique clients tracked by the
	// per-client rate limiter. When exceeded, new clients share a stricter overflow
	// limiter instead of getting individual buckets. This caps memory usage on
	// horizontally scaled Cloud Run instances.
	MaxRateLimitClients int64 `mapstructure:"MAX_RATE_LIMIT_CLIENTS"`
	// AllowedContentTypes is a list of permitted MIME types for uploads. If empty,
	// all content types are allowed.
	AllowedContentTypes []string `mapstructure:"ALLOWED_CONTENT_TYPES"`
	// MaxFileSizeBytes is the maximum allowed file size in bytes. If zero, no limit is enforced.
	MaxFileSizeBytes int64 `mapstructure:"MAX_FILE_SIZE_BYTES"`

	CorsAllowedOrigins   []string `mapstructure:"CORS_ALLOWED_ORIGINS"`
	CorsAllowCredentials bool     `mapstructure:"CORS_ALLOW_CREDENTIALS"`

	MaxRetryAttempts      int `mapstructure:"MAX_RETRY_ATTEMPTS"`
	MaxElapsedTimeSeconds int `mapstructure:"MAX_ELAPSED_TIME_SECONDS"`

	// CBMaxRequests is the maximum number of requests allowed to pass through
	// when the circuit breaker is half-open. Default: 1.
	CBMaxRequests uint32 `mapstructure:"CB_MAX_REQUESTS"`
	// CBIntervalSeconds is the cyclic period of the closed state for the
	// circuit breaker to clear the internal counts. Default: 60.
	CBIntervalSeconds int `mapstructure:"CB_INTERVAL_SECONDS"`
	// CBTimeoutSeconds is the period of the open state after which the state
	// of the circuit breaker becomes half-open. Default: 30.
	CBTimeoutSeconds int `mapstructure:"CB_TIMEOUT_SECONDS"`
	// CBConsecutiveFailures is the number of consecutive failures that trips
	// the circuit breaker from closed to open. Default: 5.
	CBConsecutiveFailures uint32 `mapstructure:"CB_CONSECUTIVE_FAILURES"`
}

// LoadConfig reads configuration from a YAML file at the given path and merges
// it with environment variables. Sensible defaults are provided for local
// development. Returns an error if the config file exists but cannot be parsed,
// or if the values cannot be unmarshalled into the Config struct.
func LoadConfig(path string) (*Config, error) {

	// Set sensible defaults for local development.
	viper.SetDefault("APP_ENV", "development")
	viper.SetDefault("HTTP_PORT", ":8080")
	viper.SetDefault("GCP_PROJECT_ID", "amith-testing")
	viper.SetDefault("FIRESTORE_DATABASE_ID", "apex-firestore-db")
	viper.SetDefault("GCS_BUCKET", "")
	viper.SetDefault("SERVICE_ACCOUNT_EMAIL", "")
	viper.SetDefault("JWT_PUBLIC_KEY_PATH", "")
	viper.SetDefault("OTEL_SERVICE_NAME", "prj-apex-upload-platform")
	viper.SetDefault("OTEL_EXPORTER_OTLP_HEADERS", "x-goog-user-project=amith-testing")
	viper.SetDefault("RATE_LIMIT_RPS", 100)
	viper.SetDefault("RATE_LIMIT_BURST", 200)
	viper.SetDefault("PER_CLIENT_RATE_LIMIT_RPS", 20)
	viper.SetDefault("PER_CLIENT_RATE_LIMIT_BURST", 40)
	viper.SetDefault("MAX_RATE_LIMIT_CLIENTS", 50000)
	viper.SetDefault("ALLOWED_CONTENT_TYPES", []string{})
	viper.SetDefault("MAX_FILE_SIZE_BYTES", int64(0))
	viper.SetDefault("CORS_ALLOWED_ORIGINS", []string{"*"})
	viper.SetDefault("CORS_ALLOW_CREDENTIALS", true)
	viper.SetDefault("MAX_RETRY_ATTEMPTS", 5)
	viper.SetDefault("MAX_ELAPSED_TIME_SECONDS", 30)
	viper.SetDefault("CB_MAX_REQUESTS", 1)
	viper.SetDefault("CB_INTERVAL_SECONDS", 60)
	viper.SetDefault("CB_TIMEOUT_SECONDS", 30)
	viper.SetDefault("CB_CONSECUTIVE_FAILURES", 5)

	viper.AddConfigPath(path)
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	// Bind environment variables; dots in keys become underscores (e.g., GCS.BUCKET -> GCS_BUCKET).
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Read the config file; ignore "file not found" since env vars may supply all values.
	err := viper.ReadInConfig()
	if _, ok := err.(viper.ConfigFileNotFoundError); err != nil && !ok {
		return nil, fmt.Errorf("fatal error config file: %w", err)
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("unable to decode into struct, %w", err)
	}

	return &config, nil
}
