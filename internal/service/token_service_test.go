package service

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"errors"
	"testing"
	"time"

	"github.com/AmithSAI007/prj-apex-upload-platform/internal/config"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

func generateTestKeyPair(t *testing.T) (*ecdsa.PrivateKey, *ecdsa.PublicKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate ECDSA key: %v", err)
	}
	return key, &key.PublicKey
}

func createSignedToken(t *testing.T, key *ecdsa.PrivateKey, claims TokenClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	tokenStr, err := token.SignedString(key)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return tokenStr
}

func newTokenService(t *testing.T, publicKey *ecdsa.PublicKey) *TokenService {
	t.Helper()
	return NewTokenService(zap.NewNop(), &config.Config{}, publicKey)
}

func TestNewTokenService(t *testing.T) {
	_, pub := generateTestKeyPair(t)
	svc := newTokenService(t, pub)
	if svc == nil {
		t.Fatal("expected non-nil TokenService")
	}
}

func TestValidateToken_ValidAccessToken(t *testing.T) {
	priv, pub := generateTestKeyPair(t)
	svc := newTokenService(t, pub)

	tokenStr := createSignedToken(t, priv, TokenClaims{
		UserID: "user_1",
		Email:  "user@example.com",
		Type:   AccessTokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(10 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	})

	claims, err := svc.ValidateToken(tokenStr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims.UserID != "user_1" {
		t.Fatalf("expected UserID 'user_1', got %s", claims.UserID)
	}
	if claims.Email != "user@example.com" {
		t.Fatalf("expected Email 'user@example.com', got %s", claims.Email)
	}
	if claims.Type != AccessTokenType {
		t.Fatalf("expected Type %s, got %s", AccessTokenType, claims.Type)
	}
}

func TestValidateToken_ExpiredToken(t *testing.T) {
	priv, pub := generateTestKeyPair(t)
	svc := newTokenService(t, pub)

	tokenStr := createSignedToken(t, priv, TokenClaims{
		UserID: "user_1",
		Email:  "user@example.com",
		Type:   AccessTokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-10 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-20 * time.Minute)),
		},
	})

	_, err := svc.ValidateToken(tokenStr)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
	if !errors.Is(err, ErrExpiredToken) {
		t.Fatalf("expected ErrExpiredToken, got %v", err)
	}
}

func TestValidateToken_WrongSigningMethod_HMAC(t *testing.T) {
	_, pub := generateTestKeyPair(t)
	svc := newTokenService(t, pub)

	// Sign with HMAC instead of ECDSA
	claims := TokenClaims{
		UserID: "user_1",
		Type:   AccessTokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(10 * time.Minute)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte("some-hmac-secret"))
	if err != nil {
		t.Fatalf("failed to sign HMAC token: %v", err)
	}

	_, err = svc.ValidateToken(tokenStr)
	if err == nil {
		t.Fatal("expected error for HMAC-signed token")
	}
	if !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("expected ErrInvalidSignature, got %v", err)
	}
}

func TestValidateToken_InvalidSignature_DifferentKey(t *testing.T) {
	priv1, _ := generateTestKeyPair(t)
	_, pub2 := generateTestKeyPair(t)
	svc := newTokenService(t, pub2)

	tokenStr := createSignedToken(t, priv1, TokenClaims{
		UserID: "user_1",
		Type:   AccessTokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(10 * time.Minute)),
		},
	})

	_, err := svc.ValidateToken(tokenStr)
	if err == nil {
		t.Fatal("expected error for token signed with different key")
	}
	if !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("expected ErrInvalidSignature, got %v", err)
	}
}

func TestValidateToken_RefreshTokenType(t *testing.T) {
	priv, pub := generateTestKeyPair(t)
	svc := newTokenService(t, pub)

	tokenStr := createSignedToken(t, priv, TokenClaims{
		UserID: "user_1",
		Type:   RefreshTokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(10 * time.Minute)),
		},
	})

	_, err := svc.ValidateToken(tokenStr)
	if err == nil {
		t.Fatal("expected error for refresh token type")
	}
	if !errors.Is(err, ErrInvalidTokenType) {
		t.Fatalf("expected ErrInvalidTokenType, got %v", err)
	}
}

func TestValidateToken_MalformedToken(t *testing.T) {
	_, pub := generateTestKeyPair(t)
	svc := newTokenService(t, pub)

	_, err := svc.ValidateToken("not.a.valid.jwt")
	if err == nil {
		t.Fatal("expected error for malformed token")
	}
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func TestValidateToken_EmptyToken(t *testing.T) {
	_, pub := generateTestKeyPair(t)
	svc := newTokenService(t, pub)

	_, err := svc.ValidateToken("")
	if err == nil {
		t.Fatal("expected error for empty token")
	}
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func TestValidateToken_CustomTokenType(t *testing.T) {
	priv, pub := generateTestKeyPair(t)
	svc := newTokenService(t, pub)

	tokenStr := createSignedToken(t, priv, TokenClaims{
		UserID: "user_1",
		Type:   "custom",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(10 * time.Minute)),
		},
	})

	_, err := svc.ValidateToken(tokenStr)
	if err == nil {
		t.Fatal("expected error for custom token type")
	}
	if !errors.Is(err, ErrInvalidTokenType) {
		t.Fatalf("expected ErrInvalidTokenType, got %v", err)
	}
}

func TestTokenServiceImplementsInterface(t *testing.T) {
	var _ TokenInterface = (*TokenService)(nil)
}
