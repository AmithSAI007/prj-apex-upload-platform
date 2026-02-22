package service

import (
	"crypto/ecdsa"
	"errors"
	"fmt"

	"github.com/AmithSAI007/prj-apex-upload-platform/internal/config"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

type TokenClaims struct {
	UserID string `json:"userId"`
	Email  string `json:"email"`
	Type   string `json:"type"`
	jwt.RegisteredClaims
}

const (
	AccessTokenType  = "access"
	RefreshTokenType = "refresh"
)

type TokenInterface interface {
	ValidateToken(tokenStr string) (*TokenClaims, error)
}

type TokenService struct {
	logger    *zap.Logger
	cfg       *config.Config
	publicKey *ecdsa.PublicKey
}

var (
	ErrInvalidToken     = errors.New("token: invalid")
	ErrInvalidTokenType = errors.New("token: invalid type")
	ErrExpiredToken     = errors.New("token: expired")
	ErrInvalidSignature = errors.New("token: invalid signature")
)

func NewTokenService(logger *zap.Logger, cfg *config.Config, publicKey *ecdsa.PublicKey) *TokenService {
	return &TokenService{
		logger:    logger,
		cfg:       cfg,
		publicKey: publicKey,
	}
}

var _ TokenInterface = (*TokenService)(nil)

func (s *TokenService) ValidateToken(tokenStr string) (*TokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodECDSA); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return s.publicKey, nil
	})

	if err != nil {
		switch {
		case errors.Is(err, jwt.ErrTokenExpired):
			s.logger.Warn("Token expired", zap.Error(err))
			return nil, fmt.Errorf("%w: %v", ErrExpiredToken, err)
		case errors.Is(err, jwt.ErrTokenSignatureInvalid) || errors.Is(err, jwt.ErrSignatureInvalid):
			s.logger.Warn("Invalid token signature", zap.Error(err))
			return nil, fmt.Errorf("%w: %v", ErrInvalidSignature, err)
		default:
			s.logger.Error("Failed to parse token", zap.Error(err))
			return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
		}
	}

	if claims, ok := token.Claims.(*TokenClaims); ok && token.Valid {
		if claims.Type != AccessTokenType {
			s.logger.Warn("Invalid token type", zap.String("type", claims.Type))
			return nil, fmt.Errorf("%w: %s", ErrInvalidTokenType, claims.Type)
		}
		return claims, nil
	}
	s.logger.Error("Invalid token claims")
	return nil, fmt.Errorf("%w", ErrInvalidToken)
}
