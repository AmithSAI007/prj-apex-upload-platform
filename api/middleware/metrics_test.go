package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestPrometheusMetrics_RecordsWithoutPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(PrometheusMetrics())
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
	if resp.Body.String() != "ok" {
		t.Fatalf("expected 'ok', got %s", resp.Body.String())
	}
}

func TestPrometheusMetrics_MultipleRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(PrometheusMetrics())
	r.POST("/submit", func(c *gin.Context) {
		c.String(http.StatusCreated, "created")
	})

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodPost, "/submit", nil)
		resp := httptest.NewRecorder()
		r.ServeHTTP(resp, req)
		if resp.Code != http.StatusCreated {
			t.Fatalf("request %d: expected 201, got %d", i, resp.Code)
		}
	}
}

func TestPrometheusMetrics_RecordsDifferentStatusCodes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(PrometheusMetrics())
	r.GET("/ok", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})
	r.GET("/not-found", func(c *gin.Context) {
		c.String(http.StatusNotFound, "not found")
	})
	r.GET("/error", func(c *gin.Context) {
		c.String(http.StatusInternalServerError, "error")
	})

	tests := []struct {
		path       string
		wantStatus int
	}{
		{"/ok", 200},
		{"/not-found", 404},
		{"/error", 500},
	}

	for _, tt := range tests {
		req := httptest.NewRequest(http.MethodGet, tt.path, nil)
		resp := httptest.NewRecorder()
		r.ServeHTTP(resp, req)
		if resp.Code != tt.wantStatus {
			t.Fatalf("path %s: expected %d, got %d", tt.path, tt.wantStatus, resp.Code)
		}
	}
}
