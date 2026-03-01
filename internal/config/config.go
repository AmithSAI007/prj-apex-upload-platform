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
}

// LoadConfig reads configuration from a YAML file at the given path and merges
// it with environment variables. Sensible defaults are provided for local
// development. Returns an error if the config file exists but cannot be parsed,
// or if the values cannot be unmarshalled into the Config struct.
func LoadConfig(path string) (*Config, error) {

	// Set sensible defaults for local development.
	viper.SetDefault("APP_ENV", "development")
	viper.SetDefault("HTTP_PORT", "8080")
	viper.SetDefault("GCP_PROJECT_ID", "amith-testing")
	viper.SetDefault("FIRESTORE_DATABASE_ID", "apex-firestore-db")
	viper.SetDefault("GCS_BUCKET", "")
	viper.SetDefault("SERVICE_ACCOUNT_EMAIL", "")
	viper.SetDefault("JWT_PUBLIC_KEY_PATH", "")
	viper.SetDefault("OTEL_SERVICE_NAME", "prj-apex-upload-platform")
	viper.SetDefault("OTEL_EXPORTER_OTLP_HEADERS", "x-goog-user-project=amith-testing")

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
