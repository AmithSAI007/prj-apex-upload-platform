package service

import (
	"crypto/ecdsa"
	"errors"
	"fmt"

	"github.com/AmithSAI007/prj-apex-upload-platform/internal/config"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

// TokenClaims extends jwt.RegisteredClaims with application-specific fields
// that are encoded in the JWT payload by the identity provider.
type TokenClaims struct {
	// UserID is the authenticated user's unique identifier.
	UserID string `json:"userId"`
	// Email is the authenticated user's email address.
	Email string `json:"email"`
	// Type discriminates between access and refresh tokens.
	Type string `json:"type"`
	jwt.RegisteredClaims
}

const (
	// AccessTokenType is the expected token type for API authentication.
	AccessTokenType = "access"
	// RefreshTokenType is the token type used for obtaining new access tokens.
	RefreshTokenType = "refresh"
)

// TokenInterface defines JWT validation behavior. Implementations parse a raw
// JWT string, verify its signature and expiration, and return the decoded claims.
type TokenInterface interface {
	ValidateToken(tokenStr string) (*TokenClaims, error)
}

// TokenService validates JWTs using an ECDSA public key. It enforces that the
// signing method is ECDSA, the token is not expired, and the token type is "access".
type TokenService struct {
	logger    *zap.Logger
	cfg       *config.Config
	publicKey *ecdsa.PublicKey
}

// Sentinel errors for token validation failures. Callers use errors.Is to
// distinguish between different failure modes (expired, bad signature, etc.).
var (
	// ErrInvalidToken indicates the JWT could not be parsed or is otherwise malformed.
	ErrInvalidToken = errors.New("token: invalid")
	// ErrInvalidTokenType indicates the token type claim is not "access".
	ErrInvalidTokenType = errors.New("token: invalid type")
	// ErrExpiredToken indicates the JWT has passed its exp claim.
	ErrExpiredToken = errors.New("token: expired")
	// ErrInvalidSignature indicates the JWT signature verification failed.
	ErrInvalidSignature = errors.New("token: invalid signature")
)

// NewTokenService constructs a TokenService with the given logger, config, and
// ECDSA public key used for JWT signature verification.
func NewTokenService(logger *zap.Logger, cfg *config.Config, publicKey *ecdsa.PublicKey) *TokenService {
	return &TokenService{
		logger:    logger,
		cfg:       cfg,
		publicKey: publicKey,
	}
}

var _ TokenInterface = (*TokenService)(nil)

// ValidateToken parses and validates a JWT string. It verifies:
//  1. The signing method is ECDSA (rejects HMAC, RSA, etc.).
//  2. The signature is valid against the configured public key.
//  3. The token is not expired.
//  4. The token type claim is "access" (rejects refresh tokens).
//
// Returns the decoded claims on success, or a wrapped sentinel error on failure.
func (s *TokenService) ValidateToken(tokenStr string) (*TokenClaims, error) {
	// Parse the token and verify its signature using the ECDSA public key.
	token, err := jwt.ParseWithClaims(tokenStr, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Reject non-ECDSA signing methods to prevent algorithm confusion attacks.
		if _, ok := token.Method.(*jwt.SigningMethodECDSA); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return s.publicKey, nil
	})

	if err != nil {
		// Classify the parse error into the appropriate sentinel error.
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

	// Extract claims and enforce access-token-only policy.
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
