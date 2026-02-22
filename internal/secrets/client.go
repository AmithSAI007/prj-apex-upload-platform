package secrets

import (
	"context"
	"fmt"
	"hash/crc32"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"go.uber.org/zap"
)

type SecretsClient struct {
	logger *zap.Logger
	client *secretmanager.Client
}

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

func (s *SecretsClient) Close() error {
	return s.client.Close()
}

func (s *SecretsClient) GetSecret(ctx context.Context, secretName string) (string, error) {
	accessReq := &secretmanagerpb.AccessSecretVersionRequest{
		Name: fmt.Sprintf("%s/versions/latest", secretName),
	}

	accessResp, err := s.client.AccessSecretVersion(ctx, accessReq)
	if err != nil {
		s.logger.Error("Failed to access secret version", zap.String("secretName", secretName), zap.Error(err))
		return "", fmt.Errorf("failed to access secret version: %w", err)
	}

	crc32c := crc32.MakeTable(crc32.Castagnoli)
	checksum := int64(crc32.Checksum(accessResp.Payload.Data, crc32c))
	if checksum != *accessResp.Payload.DataCrc32C {
		s.logger.Error("Data integrity check failed: checksum mismatch", zap.String("secretName", secretName))
		return "", fmt.Errorf("data integrity check failed: checksum mismatch")
	}
	s.logger.Debug("Successfully accessed secret", zap.String("secretName", secretName))
	return string(accessResp.Payload.Data), nil

}
