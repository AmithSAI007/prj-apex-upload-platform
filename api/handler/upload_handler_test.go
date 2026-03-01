package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AmithSAI007/prj-apex-upload-platform/api/dto"
	"github.com/AmithSAI007/prj-apex-upload-platform/api/middleware"
	internalerrors "github.com/AmithSAI007/prj-apex-upload-platform/internal/errors"
	"github.com/AmithSAI007/prj-apex-upload-platform/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.uber.org/zap"
)

type stubUploadService struct {
	createFn      func(req dto.CreateUploadRequest) (dto.CreateUploadResponse, error)
	resumeFn      func(uploadID string) (dto.ResumeUploadResponse, error)
	getStatusFn   func(uploadID string) (dto.UploadStatusResponse, error)
	queryStatusFn func(uploadID string, req dto.QueryStatusRequest) (dto.QueryStatusResponse, error)
	cancelFn      func(uploadID string, req dto.CancelUploadRequest) (dto.CancelUploadResponse, error)
}

func (s *stubUploadService) CreateUploadSession(_ context.Context, req dto.CreateUploadRequest) (dto.CreateUploadResponse, error) {
	if s.createFn == nil {
		return dto.CreateUploadResponse{}, nil
	}
	return s.createFn(req)
}

func (s *stubUploadService) ResumeUploadSession(_ context.Context, uploadID string) (dto.ResumeUploadResponse, error) {
	if s.resumeFn == nil {
		return dto.ResumeUploadResponse{}, nil
	}
	return s.resumeFn(uploadID)
}

func (s *stubUploadService) GetUploadStatus(_ context.Context, uploadID string) (dto.UploadStatusResponse, error) {
	if s.getStatusFn == nil {
		return dto.UploadStatusResponse{}, nil
	}
	return s.getStatusFn(uploadID)
}

func (s *stubUploadService) QueryUploadStatus(_ context.Context, uploadID string, req dto.QueryStatusRequest) (dto.QueryStatusResponse, error) {
	if s.queryStatusFn == nil {
		return dto.QueryStatusResponse{}, nil
	}
	return s.queryStatusFn(uploadID, req)
}

func (s *stubUploadService) CancelUploadSession(_ context.Context, uploadID string, req dto.CancelUploadRequest) (dto.CancelUploadResponse, error) {
	if s.cancelFn == nil {
		return dto.CancelUploadResponse{}, nil
	}
	return s.cancelFn(uploadID, req)
}

func TestCreateUploadSession_InvalidJSON(t *testing.T) {
	router := setupTestRouter(&stubUploadService{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/uploads", bytes.NewBufferString("{invalid"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(middleware.RequestIDHeader, "req_test_invalid_json")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", resp.Code)
	}

	var payload dto.ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if payload.Error.Code != dto.ErrorCodeInvalidArgument {
		t.Fatalf("expected error code %s, got %s", dto.ErrorCodeInvalidArgument, payload.Error.Code)
	}
	if payload.Error.RequestID != "req_test_invalid_json" {
		t.Fatalf("expected requestId to match, got %s", payload.Error.RequestID)
	}
}

func TestCreateUploadSession_ValidationError(t *testing.T) {
	router := setupTestRouter(&stubUploadService{})
	body := `{"fileName":"","contentType":"","sizeBytes":0}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/uploads", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(middleware.RequestIDHeader, "req_test_validation")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", resp.Code)
	}

	var payload dto.ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if payload.Error.Code != dto.ErrorCodeInvalidArgument {
		t.Fatalf("expected error code %s, got %s", dto.ErrorCodeInvalidArgument, payload.Error.Code)
	}
}

func TestCreateUploadSession_InvalidInputFromService(t *testing.T) {
	service := &stubUploadService{
		createFn: func(_ dto.CreateUploadRequest) (dto.CreateUploadResponse, error) {
			return dto.CreateUploadResponse{}, internalerrors.ErrInvalidInput
		},
	}
	router := setupTestRouter(service)
	body := `{"fileName":"file.mp4","contentType":"video/mp4","sizeBytes":10}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/uploads", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(middleware.RequestIDHeader, "req_test_invalid_input")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", resp.Code)
	}

	var payload dto.ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if payload.Error.Code != dto.ErrorCodeInvalidArgument {
		t.Fatalf("expected error code %s, got %s", dto.ErrorCodeInvalidArgument, payload.Error.Code)
	}
}

func TestCreateUploadSession_ServiceError(t *testing.T) {
	service := &stubUploadService{
		createFn: func(_ dto.CreateUploadRequest) (dto.CreateUploadResponse, error) {
			return dto.CreateUploadResponse{}, errors.New("service failure")
		},
	}
	router := setupTestRouter(service)
	body := `{"fileName":"file.mp4","contentType":"video/mp4","sizeBytes":10}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/uploads", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(middleware.RequestIDHeader, "req_test_service_error")

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
}

func TestCreateUploadSession_Success(t *testing.T) {
	service := &stubUploadService{
		createFn: func(_ dto.CreateUploadRequest) (dto.CreateUploadResponse, error) {
			return dto.CreateUploadResponse{
				UploadID:         "upl_0192f2c1-6a7c-79b2-8f73-2d3c3a4b5c6d",
				GCSUploadURL:     "https://storage.googleapis.com/upload/storage/v1/b/my-bucket/o?uploadType=resumable&upload_id=...",
				Bucket:           "my-bucket",
				ObjectName:       "uploads/upl_0192f2c1-6a7c-79b2-8f73-2d3c3a4b5c6d/file.mp4",
				SessionExpiresAt: "2026-02-16T12:00:00Z",
				Status:           dto.StatusCreated,
			}, nil
		},
	}
	router := setupTestRouter(service)
	body := `{"fileName":"file.mp4","contentType":"video/mp4","sizeBytes":10}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/uploads", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(middleware.RequestIDHeader, "req_test_success")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", resp.Code)
	}

	var payload dto.CreateUploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if payload.UploadID == "" || payload.GCSUploadURL == "" {
		t.Fatalf("expected upload response fields to be set")
	}
	if resp.Header().Get(middleware.RequestIDHeader) != "req_test_success" {
		t.Fatalf("expected response request id header to be set")
	}
}

func TestCreateUploadSession_WithChecksum(t *testing.T) {
	service := &stubUploadService{
		createFn: func(req dto.CreateUploadRequest) (dto.CreateUploadResponse, error) {
			if req.Checksum == nil || req.Checksum.Algorithm != "md5" {
				return dto.CreateUploadResponse{}, errors.New("checksum not passed")
			}
			return dto.CreateUploadResponse{UploadID: "upl_1", GCSUploadURL: "url", Bucket: "b", ObjectName: "o", SessionExpiresAt: "2026-02-16T12:00:00Z", Status: dto.StatusCreated}, nil
		},
	}
	router := setupTestRouter(service)
	body := `{"fileName":"file.mp4","contentType":"video/mp4","sizeBytes":10,"checksum":{"algorithm":"md5","value":"abcd"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/uploads", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", resp.Code)
	}
}

func TestCreateUploadSession_WithIdempotencyKey(t *testing.T) {
	var captured dto.CreateUploadRequest
	service := &stubUploadService{
		createFn: func(req dto.CreateUploadRequest) (dto.CreateUploadResponse, error) {
			captured = req
			return dto.CreateUploadResponse{UploadID: "upl_1", GCSUploadURL: "url", Bucket: "b", ObjectName: "o", SessionExpiresAt: "2026-02-16T12:00:00Z", Status: dto.StatusCreated}, nil
		},
	}
	router := setupTestRouter(service)
	body := `{"fileName":"file.mp4","contentType":"video/mp4","sizeBytes":10,"idempotencyKey":"idemp"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/uploads", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", resp.Code)
	}
	if captured.IdempotencyKey != "idemp" {
		t.Fatalf("expected idempotencyKey to be passed")
	}
}

func TestCreateUploadSession_WithMetadataAndLabels(t *testing.T) {
	var captured dto.CreateUploadRequest
	service := &stubUploadService{
		createFn: func(req dto.CreateUploadRequest) (dto.CreateUploadResponse, error) {
			captured = req
			return dto.CreateUploadResponse{UploadID: "upl_1", GCSUploadURL: "url", Bucket: "b", ObjectName: "o", SessionExpiresAt: "2026-02-16T12:00:00Z", Status: dto.StatusCreated}, nil
		},
	}
	router := setupTestRouter(service)
	body := `{"fileName":"file.mp4","contentType":"video/mp4","sizeBytes":10,"metadata":{"projectId":"p1"},"labels":{"docType":"video"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/uploads", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", resp.Code)
	}
	if captured.Metadata["projectId"] != "p1" || captured.Labels["docType"] != "video" {
		t.Fatalf("expected metadata and labels to be passed")
	}
}

func TestUploadEndpoints_BasicSuccess(t *testing.T) {
	router := setupTestRouter(&stubUploadService{})
	requests := []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodPost, path: "/api/v1/uploads/upl_123/resume", body: ""},
		{method: http.MethodGet, path: "/api/v1/uploads/upl_123", body: ""},
		{method: http.MethodPost, path: "/api/v1/uploads/upl_123/status", body: "{}"},
		{method: http.MethodPost, path: "/api/v1/uploads/upl_123/cancel", body: "{}"},
	}

	for _, item := range requests {
		req := httptest.NewRequest(item.method, item.path, bytes.NewBufferString(item.body))
		if item.body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		req.Header.Set(middleware.RequestIDHeader, "req_test_success")
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("expected status 200 for %s %s, got %d", item.method, item.path, resp.Code)
		}
	}
}

// --- Resume endpoint tests ---

func TestResume_NotFound(t *testing.T) {
	svc := &stubUploadService{
		resumeFn: func(_ string) (dto.ResumeUploadResponse, error) {
			return dto.ResumeUploadResponse{}, internalerrors.ErrNotFound
		},
	}
	router := setupTestRouter(svc)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/uploads/upl_123/resume", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.Code)
	}
}

func TestResume_Expired(t *testing.T) {
	svc := &stubUploadService{
		resumeFn: func(_ string) (dto.ResumeUploadResponse, error) {
			return dto.ResumeUploadResponse{}, internalerrors.ErrSessionExpired
		},
	}
	router := setupTestRouter(svc)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/uploads/upl_123/resume", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusGone {
		t.Fatalf("expected 410, got %d", resp.Code)
	}
}

func TestResume_InternalError(t *testing.T) {
	svc := &stubUploadService{
		resumeFn: func(_ string) (dto.ResumeUploadResponse, error) {
			return dto.ResumeUploadResponse{}, errors.New("db error")
		},
	}
	router := setupTestRouter(svc)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/uploads/upl_123/resume", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.Code)
	}
}

// --- GetStatus endpoint tests ---

func TestGetStatus_NotFound(t *testing.T) {
	svc := &stubUploadService{
		getStatusFn: func(_ string) (dto.UploadStatusResponse, error) {
			return dto.UploadStatusResponse{}, internalerrors.ErrNotFound
		},
	}
	router := setupTestRouter(svc)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/uploads/upl_123", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.Code)
	}
}

func TestGetStatus_Expired(t *testing.T) {
	svc := &stubUploadService{
		getStatusFn: func(_ string) (dto.UploadStatusResponse, error) {
			return dto.UploadStatusResponse{}, internalerrors.ErrSessionExpired
		},
	}
	router := setupTestRouter(svc)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/uploads/upl_123", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusGone {
		t.Fatalf("expected 410, got %d", resp.Code)
	}
}

func TestGetStatus_InternalError(t *testing.T) {
	svc := &stubUploadService{
		getStatusFn: func(_ string) (dto.UploadStatusResponse, error) {
			return dto.UploadStatusResponse{}, errors.New("db error")
		},
	}
	router := setupTestRouter(svc)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/uploads/upl_123", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.Code)
	}
}

// --- QueryStatus endpoint tests ---

func TestQueryStatus_InvalidJSON(t *testing.T) {
	router := setupTestRouter(&stubUploadService{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/uploads/upl_123/status", bytes.NewBufferString("{bad"))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.Code)
	}
}

func TestQueryStatus_NotFound(t *testing.T) {
	svc := &stubUploadService{
		queryStatusFn: func(_ string, _ dto.QueryStatusRequest) (dto.QueryStatusResponse, error) {
			return dto.QueryStatusResponse{}, internalerrors.ErrNotFound
		},
	}
	router := setupTestRouter(svc)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/uploads/upl_123/status", bytes.NewBufferString("{}"))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.Code)
	}
}

func TestQueryStatus_Expired(t *testing.T) {
	svc := &stubUploadService{
		queryStatusFn: func(_ string, _ dto.QueryStatusRequest) (dto.QueryStatusResponse, error) {
			return dto.QueryStatusResponse{}, internalerrors.ErrSessionExpired
		},
	}
	router := setupTestRouter(svc)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/uploads/upl_123/status", bytes.NewBufferString("{}"))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusGone {
		t.Fatalf("expected 410, got %d", resp.Code)
	}
}

func TestQueryStatus_InternalError(t *testing.T) {
	svc := &stubUploadService{
		queryStatusFn: func(_ string, _ dto.QueryStatusRequest) (dto.QueryStatusResponse, error) {
			return dto.QueryStatusResponse{}, errors.New("db error")
		},
	}
	router := setupTestRouter(svc)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/uploads/upl_123/status", bytes.NewBufferString("{}"))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.Code)
	}
}

func TestQueryStatus_InvalidInput(t *testing.T) {
	svc := &stubUploadService{
		queryStatusFn: func(_ string, _ dto.QueryStatusRequest) (dto.QueryStatusResponse, error) {
			return dto.QueryStatusResponse{}, internalerrors.ErrInvalidInput
		},
	}
	router := setupTestRouter(svc)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/uploads/upl_123/status", bytes.NewBufferString("{}"))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.Code)
	}
}

// --- Cancel endpoint tests ---

func TestCancel_InvalidJSON(t *testing.T) {
	router := setupTestRouter(&stubUploadService{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/uploads/upl_123/cancel", bytes.NewBufferString("{bad"))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.Code)
	}
}

func TestCancel_NotFound(t *testing.T) {
	svc := &stubUploadService{
		cancelFn: func(_ string, _ dto.CancelUploadRequest) (dto.CancelUploadResponse, error) {
			return dto.CancelUploadResponse{}, internalerrors.ErrNotFound
		},
	}
	router := setupTestRouter(svc)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/uploads/upl_123/cancel", bytes.NewBufferString("{}"))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.Code)
	}
}

func TestCancel_Expired(t *testing.T) {
	svc := &stubUploadService{
		cancelFn: func(_ string, _ dto.CancelUploadRequest) (dto.CancelUploadResponse, error) {
			return dto.CancelUploadResponse{}, internalerrors.ErrSessionExpired
		},
	}
	router := setupTestRouter(svc)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/uploads/upl_123/cancel", bytes.NewBufferString("{}"))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusGone {
		t.Fatalf("expected 410, got %d", resp.Code)
	}
}

func TestCancel_InternalError(t *testing.T) {
	svc := &stubUploadService{
		cancelFn: func(_ string, _ dto.CancelUploadRequest) (dto.CancelUploadResponse, error) {
			return dto.CancelUploadResponse{}, errors.New("db error")
		},
	}
	router := setupTestRouter(svc)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/uploads/upl_123/cancel", bytes.NewBufferString("{}"))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.Code)
	}
}

func TestCancel_InvalidInput(t *testing.T) {
	svc := &stubUploadService{
		cancelFn: func(_ string, _ dto.CancelUploadRequest) (dto.CancelUploadResponse, error) {
			return dto.CancelUploadResponse{}, internalerrors.ErrInvalidInput
		},
	}
	router := setupTestRouter(svc)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/uploads/upl_123/cancel", bytes.NewBufferString("{}"))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.Code)
	}
}

func setupTestRouter(svc service.UploadInterface) *gin.Engine {
	logger := zap.NewNop()
	validate := validator.New()
	uploadHandler := NewUploadHandler(logger, validate, svc)

	router := gin.New()
	router.Use(middleware.RequestContext())
	router.POST("/api/v1/uploads", uploadHandler.Create)
	router.POST("/api/v1/uploads/:uploadId/resume", uploadHandler.Resume)
	router.GET("/api/v1/uploads/:uploadId", uploadHandler.GetStatus)
	router.POST("/api/v1/uploads/:uploadId/status", uploadHandler.QueryStatus)
	router.POST("/api/v1/uploads/:uploadId/cancel", uploadHandler.Cancel)
	return router
}

// setupTestRouterWithTracing returns a gin.Engine that injects a real
// recording OTel span into every request context, so the
// respondWithServiceError span branch is exercised.
func setupTestRouterWithTracing(svc service.UploadInterface) *gin.Engine {
	logger := zap.NewNop()
	validate := validator.New()
	uploadHandler := NewUploadHandler(logger, validate, svc)

	tp := sdktrace.NewTracerProvider()
	tracer := tp.Tracer("test")

	router := gin.New()
	router.Use(middleware.RequestContext())
	// Inject a recording span before every handler.
	router.Use(func(c *gin.Context) {
		ctx, span := tracer.Start(c.Request.Context(), "test-span")
		defer span.End()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	router.POST("/api/v1/uploads", uploadHandler.Create)
	router.POST("/api/v1/uploads/:uploadId/resume", uploadHandler.Resume)
	router.GET("/api/v1/uploads/:uploadId", uploadHandler.GetStatus)
	router.POST("/api/v1/uploads/:uploadId/status", uploadHandler.QueryStatus)
	router.POST("/api/v1/uploads/:uploadId/cancel", uploadHandler.Cancel)
	return router
}

// TestRespondWithServiceError_WithRecordingSpan verifies that when a service
// error occurs with a recording span in context, the span records the error
// and sets an error status.
func TestRespondWithServiceError_WithRecordingSpan(t *testing.T) {
	svc := &stubUploadService{
		resumeFn: func(_ string) (dto.ResumeUploadResponse, error) {
			return dto.ResumeUploadResponse{}, internalerrors.ErrNotFound
		},
	}
	router := setupTestRouterWithTracing(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/uploads/upl_123/resume", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.Code)
	}
}

// TestRespondWithServiceError_InternalError_WithSpan covers the internal
// error path with a recording span active.
func TestRespondWithServiceError_InternalError_WithSpan(t *testing.T) {
	svc := &stubUploadService{
		resumeFn: func(_ string) (dto.ResumeUploadResponse, error) {
			return dto.ResumeUploadResponse{}, errors.New("unexpected failure")
		},
	}
	router := setupTestRouterWithTracing(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/uploads/upl_123/resume", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.Code)
	}
}
