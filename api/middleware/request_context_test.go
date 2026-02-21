package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequestContext_UsesIncomingRequestID(t *testing.T) {
	router := gin.New()
	router.Use(RequestContext())
	router.GET("/", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"trace": TraceIDFromContext(ctx.Request.Context())})
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(RequestIDHeader, "req_test_123")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Header().Get(RequestIDHeader) != "req_test_123" {
		t.Fatalf("expected response to echo request id header")
	}
}

func TestRequestContext_GeneratesRequestID(t *testing.T) {
	router := gin.New()
	router.Use(RequestContext())
	router.GET("/", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"trace": TraceIDFromContext(ctx.Request.Context())})
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	requestID := resp.Header().Get(RequestIDHeader)
	if requestID == "" {
		t.Fatalf("expected response to include request id header")
	}
}
