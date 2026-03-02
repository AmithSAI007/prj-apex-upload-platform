// Package secrets provides a thin wrapper around the GCP Secret Manager API.
// It is used at startup to load sensitive configuration values (e.g., ECDSA
// public keys for JWT verification) with CRC32C integrity checking.
package secrets

import (
	"context"
	"fmt"
	"hash/crc32"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"go.uber.org/zap"
)

// SecretsClient wraps the GCP Secret Manager client and provides a simplified
// interface for retrieving secret values with data integrity verification.
type SecretsClient struct {
	logger *zap.Logger
	client *secretmanager.Client
}

// NewSecretsClient creates a new Secret Manager client using Application Default
// Credentials. Returns an error if the underlying gRPC connection cannot be established.
func NewSecretsClient(ctx context.Context, logger *zap.Logger) (*SecretsClient, error) {
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create Secret Manager client: %w", err)
	}
	return &SecretsClient{
		logger: logger,
		client: client,
	}, nil
}

// Close releases resources held by the Secret Manager gRPC client.
func (s *SecretsClient) Close() error {
	return s.client.Close()
}

// GetSecret fetches the latest version of the named secret from Secret Manager.
// It appends "/versions/latest" to the provided secret name, retrieves the
// payload, and verifies its CRC32C checksum to detect data corruption in transit.
// Returns the secret value as a string, or an error if the access or integrity
// check fails.
func (s *SecretsClient) GetSecret(ctx context.Context, secretName string) (string, error) {
	accessReq := &secretmanagerpb.AccessSecretVersionRequest{
		Name: fmt.Sprintf("%s/versions/latest", secretName),
	}

	accessResp, err := s.client.AccessSecretVersion(ctx, accessReq)
	if err != nil {
		s.logger.Error("Failed to access secret version", zap.String("secretName", secretName), zap.Error(err))
		return "", fmt.Errorf("failed to access secret version: %w", err)
	}

	// Verify data integrity using CRC32C (Castagnoli) checksum.
	crc32c := crc32.MakeTable(crc32.Castagnoli)
	checksum := int64(crc32.Checksum(accessResp.Payload.Data, crc32c))
	if checksum != *accessResp.Payload.DataCrc32C {
		s.logger.Error("Data integrity check failed: checksum mismatch", zap.String("secretName", secretName))
		return "", fmt.Errorf("data integrity check failed: checksum mismatch")
	}
	s.logger.Debug("Successfully accessed secret", zap.String("secretName", secretName))
	return string(accessResp.Payload.Data), nil
}
