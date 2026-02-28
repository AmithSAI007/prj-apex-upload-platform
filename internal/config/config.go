package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	AppEnv                string `mapstructure:"APP_ENV"`
	HttpPort              string `mapstructure:"HTTP_PORT"`
	GCPProjectID          string `mapstructure:"GCP_PROJECT_ID"`
	FirestoreDatabaseID   string `mapstructure:"FIRESTORE_DATABASE_ID"`
	GCSBucket             string `mapstructure:"GCS_BUCKET"`
	SERVICE_ACCOUNT_EMAIL string `mapstructure:"SERVICE_ACCOUNT_EMAIL"`
	JWT_PUBLIC_KEY_PATH   string `mapstructure:"JWT_PUBLIC_KEY_PATH"`
	OTEL_SERVICE_NAME     string `mapstructure:"OTEL_SERVICE_NAME"`
}

func LoadConfig(path string) (*Config, error) {

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

	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

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
