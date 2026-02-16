package config

import (
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func NewLogger() (*zap.Logger, error) {
	var logger *zap.Logger
	var err error

	// Check for an environment variable to determine the logging style.
	if os.Getenv("APP_ENV") == "production" {
		// Production Logger: JSON format, logs to stdout and a file.
		cfg := zap.NewProductionConfig()
		cfg.OutputPaths = []string{"stdout", "app.log"}
		cfg.ErrorOutputPaths = []string{"stderr", "app.log"}
		logger, err = cfg.Build()
	} else {
		// Development Logger: Human-readable, colorized console output.
		config := zap.NewDevelopmentEncoderConfig()

		// Customize the encoder for better readability.
		config.EncodeTime = zapcore.TimeEncoderOfLayout(time.RFC3339) // e.g., 2024-01-02T15:04:05Z07:00
		config.EncodeLevel = zapcore.CapitalColorLevelEncoder         // e.g., INFO, WARN, ERROR (with colors).

		logger = zap.New(zapcore.NewCore(
			zapcore.NewConsoleEncoder(config),                       // Use the console encoder.
			zapcore.NewMultiWriteSyncer(zapcore.AddSync(os.Stdout)), // Write to standard out.
			zap.InfoLevel, // Log all levels from Info and above.
		))
	}

	if err != nil {
		return nil, err
	}

	// Make this logger globally accessible in your application, if needed.
	// zap.ReplaceGlobals(logger)

	return logger, nil
}
