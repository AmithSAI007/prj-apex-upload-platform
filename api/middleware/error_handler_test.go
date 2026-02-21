package middleware

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AmithSAI007/prj-apex-upload-platform/internal/api/dto"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func TestErrorHandler_HandlesPanic(t *testing.T) {
	router := gin.New()
	router.Use(RequestContext())
	router.Use(ErrorHandler(zap.NewNop()))
	router.GET("/panic", func(ctx *gin.Context) {
		panic("boom")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	req.Header.Set(RequestIDHeader, "req_test_panic")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", resp.Code)
	}

	var payload dto.ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if payload.Error.Code != dto.ErrorCodeInternal {
		t.Fatalf("expected error code %s, got %s", dto.ErrorCodeInternal, payload.Error.Code)
	}
	if payload.Error.RequestID != "req_test_panic" {
		t.Fatalf("expected request id to be propagated")
	}
}

func TestErrorHandler_HandlesContextErrors(t *testing.T) {
	router := gin.New()
	router.Use(RequestContext())
	router.Use(ErrorHandler(zap.NewNop()))
	router.GET("/error", func(ctx *gin.Context) {
		_ = ctx.Error(errors.New("bad request"))
		ctx.Status(http.StatusBadRequest)
	})

	req := httptest.NewRequest(http.MethodGet, "/error", nil)
	req.Header.Set(RequestIDHeader, "req_test_error")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", resp.Code)
	}
}
