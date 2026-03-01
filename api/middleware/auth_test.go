package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AmithSAI007/prj-apex-upload-platform/api/dto"
	"github.com/AmithSAI007/prj-apex-upload-platform/internal/service"
	"github.com/AmithSAI007/prj-apex-upload-platform/pkg/constants"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"
)

// fakeTokenService implements service.TokenInterface for testing.
type fakeTokenService struct {
	claims *service.TokenClaims
	err    error
}

func (f *fakeTokenService) ValidateToken(_ string) (*service.TokenClaims, error) {
	return f.claims, f.err
}

func setupAuthRouter(tokenSvc service.TokenInterface) *gin.Engine {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()
	tracer := otel.Tracer("test")
	authMW := NewAuthMiddleware(logger, tokenSvc, tracer)

	r := gin.New()
	r.Use(RequestContext())
	r.Use(authMW.Authenticate())
	r.GET("/protected", func(c *gin.Context) {
		userID, _ := c.Request.Context().Value(constants.CtxUserIDKey).(string)
		c.JSON(http.StatusOK, gin.H{"userId": userID})
	})
	return r
}

func TestAuthenticate_MissingAuthorizationHeader(t *testing.T) {
	router := setupAuthRouter(&fakeTokenService{})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.Code)
	}
	var payload dto.ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if payload.Error.Code != dto.ErrorCodeUnauthorized {
		t.Fatalf("expected error code %s, got %s", dto.ErrorCodeUnauthorized, payload.Error.Code)
	}
	if payload.Error.Message != "Missing Authorization header" {
		t.Fatalf("expected message about missing header, got %s", payload.Error.Message)
	}
}

func TestAuthenticate_InvalidHeaderFormat_NoBearer(t *testing.T) {
	router := setupAuthRouter(&fakeTokenService{})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Basic abc123")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.Code)
	}
	var payload dto.ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if payload.Error.Message != "Invalid Authorization header format" {
		t.Fatalf("unexpected message: %s", payload.Error.Message)
	}
}

func TestAuthenticate_InvalidHeaderFormat_SinglePart(t *testing.T) {
	router := setupAuthRouter(&fakeTokenService{})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "BearerTokenWithNoSpace")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.Code)
	}
}

func TestAuthenticate_TokenExpired(t *testing.T) {
	tokenSvc := &fakeTokenService{err: service.ErrExpiredToken}
	router := setupAuthRouter(tokenSvc)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer expired-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.Code)
	}
	var payload dto.ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if payload.Error.Message != "Token expired" {
		t.Fatalf("expected 'Token expired', got %s", payload.Error.Message)
	}
}

func TestAuthenticate_InvalidSignature(t *testing.T) {
	tokenSvc := &fakeTokenService{err: service.ErrInvalidSignature}
	router := setupAuthRouter(tokenSvc)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer bad-sig-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.Code)
	}
	var payload dto.ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if payload.Error.Message != "Invalid token signature" {
		t.Fatalf("expected 'Invalid token signature', got %s", payload.Error.Message)
	}
}

func TestAuthenticate_InvalidTokenType(t *testing.T) {
	tokenSvc := &fakeTokenService{err: service.ErrInvalidTokenType}
	router := setupAuthRouter(tokenSvc)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer refresh-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.Code)
	}
	var payload dto.ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if payload.Error.Message != "Invalid token type" {
		t.Fatalf("expected 'Invalid token type', got %s", payload.Error.Message)
	}
}

func TestAuthenticate_GenericTokenError(t *testing.T) {
	tokenSvc := &fakeTokenService{err: service.ErrInvalidToken}
	router := setupAuthRouter(tokenSvc)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer malformed-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.Code)
	}
	var payload dto.ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if payload.Error.Message != "Invalid token" {
		t.Fatalf("expected 'Invalid token', got %s", payload.Error.Message)
	}
}

func TestAuthenticate_Success(t *testing.T) {
	tokenSvc := &fakeTokenService{
		claims: &service.TokenClaims{
			UserID: "user_123",
			Email:  "user@example.com",
			Type:   service.AccessTokenType,
		},
	}
	router := setupAuthRouter(tokenSvc)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer valid-access-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if body["userId"] != "user_123" {
		t.Fatalf("expected userId 'user_123', got %s", body["userId"])
	}
}

func TestAuthenticate_BearerCaseInsensitive(t *testing.T) {
	tokenSvc := &fakeTokenService{
		claims: &service.TokenClaims{
			UserID: "user_456",
			Email:  "user@example.com",
			Type:   service.AccessTokenType,
		},
	}
	router := setupAuthRouter(tokenSvc)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "bearer valid-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 (case-insensitive bearer), got %d", resp.Code)
	}
}
