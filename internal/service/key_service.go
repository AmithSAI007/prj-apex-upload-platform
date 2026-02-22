package service

import (
	"context"
	"crypto/ecdsa"

	"github.com/AmithSAI007/prj-apex-upload-platform/internal/secrets"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

type KeyInterface interface {
	LoadKey(ctx context.Context, publicKeyPath string) (*ecdsa.PublicKey, error)
}

type SMKeyService struct {
	loggger *zap.Logger
	client  *secrets.SecretsClient
}

func NewSMKeyService(logger *zap.Logger, client *secrets.SecretsClient) *SMKeyService {
	return &SMKeyService{
		loggger: logger,
		client:  client,
	}
}

var _ KeyInterface = (*SMKeyService)(nil)

func (s *SMKeyService) LoadKey(ctx context.Context, publicKeyPath string) (*ecdsa.PublicKey, error) {

	value, err := s.client.GetSecret(ctx, publicKeyPath)
	if err != nil {
		return nil, err
	}

	publicKey, err := jwt.ParseECPublicKeyFromPEM([]byte(value))
	if err != nil {
		return nil, err
	}

	return publicKey, nil
}
