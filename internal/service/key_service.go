package service

import (
	"context"
	"crypto/ecdsa"

	"github.com/AmithSAI007/prj-apex-upload-platform/internal/secrets"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

// KeyInterface defines the contract for loading cryptographic keys from an
// external key store (e.g., GCP Secret Manager).
type KeyInterface interface {
	// LoadKey fetches the PEM-encoded ECDSA public key from the given secret
	// path and returns the parsed key. Used during startup to configure JWT
	// signature verification.
	LoadKey(ctx context.Context, publicKeyPath string) (*ecdsa.PublicKey, error)
}

// SMKeyService loads ECDSA public keys from GCP Secret Manager.
type SMKeyService struct {
	loggger *zap.Logger
	client  *secrets.SecretsClient
}

// NewSMKeyService constructs an SMKeyService with the given logger and
// Secret Manager client.
func NewSMKeyService(logger *zap.Logger, client *secrets.SecretsClient) *SMKeyService {
	return &SMKeyService{
		loggger: logger,
		client:  client,
	}
}

var _ KeyInterface = (*SMKeyService)(nil)

// LoadKey retrieves the PEM-encoded public key from Secret Manager at the given
// path, parses it as an ECDSA public key, and returns it. Returns an error if
// the secret cannot be accessed or the PEM data is invalid.
func (s *SMKeyService) LoadKey(ctx context.Context, publicKeyPath string) (*ecdsa.PublicKey, error) {
	// Fetch the raw PEM bytes from Secret Manager.
	value, err := s.client.GetSecret(ctx, publicKeyPath)
	if err != nil {
		return nil, err
	}

	// Parse the PEM-encoded EC public key.
	publicKey, err := jwt.ParseECPublicKeyFromPEM([]byte(value))
	if err != nil {
		return nil, err
	}

	return publicKey, nil
}
