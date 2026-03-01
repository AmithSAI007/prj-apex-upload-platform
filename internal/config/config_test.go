package config

import (
	"os"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestLoadConfig_Defaults(t *testing.T) {
	clearEnv(t, "APP_ENV", "HTTP_PORT", "GCP_PROJECT_ID", "FIRESTORE_DATABASE_ID", "GCS_BUCKET", "SERVICE_ACCOUNT_EMAIL")
	config, err := LoadConfig(".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config.AppEnv == "" || config.HttpPort == "" {
		t.Fatalf("expected defaults to be set")
	}
	if config.FirestoreDatabaseID != "apex-firestore-db" {
		t.Fatalf("expected default Firestore database id")
	}
}

func TestLoadConfig_EnvOverride(t *testing.T) {
	setEnv(t, "APP_ENV", "test")
	setEnv(t, "HTTP_PORT", "9090")
	setEnv(t, "GCP_PROJECT_ID", "test-project")
	setEnv(t, "FIRESTORE_DATABASE_ID", "custom-db")
	setEnv(t, "GCS_BUCKET", "bucket")
	setEnv(t, "SERVICE_ACCOUNT_EMAIL", "sa@test")

	config, err := LoadConfig(".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config.AppEnv != "test" || config.HttpPort != "9090" || config.GCPProjectID != "test-project" {
		t.Fatalf("expected env overrides to be applied")
	}
	if config.FirestoreDatabaseID != "custom-db" || config.GCSBucket != "bucket" || config.SERVICE_ACCOUNT_EMAIL != "sa@test" {
		t.Fatalf("expected env overrides to be applied")
	}
}

func setEnv(t *testing.T, key string, value string) {
	t.Helper()
	if err := os.Setenv(key, value); err != nil {
		t.Fatalf("failed to set env: %v", err)
	}
}

func clearEnv(t *testing.T, keys ...string) {
	t.Helper()
	for _, key := range keys {
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("failed to clear env: %v", err)
		}
	}
}

func TestLoadConfig_MalformedYAML(t *testing.T) {
	// Reset Viper global state so prior tests don't interfere.
	viper.Reset()

	// Create a temp directory with a malformed config.yaml.
	tmpDir := t.TempDir()
	malformed := []byte("{{{invalid yaml content")
	if err := os.WriteFile(tmpDir+"/config.yaml", malformed, 0644); err != nil {
		t.Fatalf("failed to write malformed config: %v", err)
	}

	cfg, err := LoadConfig(tmpDir)
	if err == nil {
		t.Fatalf("expected error for malformed YAML, got config: %+v", cfg)
	}
	if !strings.Contains(err.Error(), "fatal error config file") {
		t.Fatalf("expected 'fatal error config file' in error, got: %v", err)
	}
}

func TestNewLogger_Development(t *testing.T) {
	clearEnv(t, "APP_ENV")
	logger, err := NewLogger()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
	// Verify logger can write without panicking.
	logger.Info("test log in development mode")
}

func TestNewLogger_Production(t *testing.T) {
	setEnv(t, "APP_ENV", "production")
	defer clearEnv(t, "APP_ENV")
	// Remove any leftover app.log from a previous test run.
	os.Remove("app.log")
	defer os.Remove("app.log")

	logger, err := NewLogger()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
	logger.Info("test log in production mode")
}
