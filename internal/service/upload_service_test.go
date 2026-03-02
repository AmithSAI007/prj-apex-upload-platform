package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/AmithSAI007/prj-apex-upload-platform/api/dto"
	"github.com/AmithSAI007/prj-apex-upload-platform/internal/config"
	internalerrors "github.com/AmithSAI007/prj-apex-upload-platform/internal/errors"
	"github.com/AmithSAI007/prj-apex-upload-platform/internal/model"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"
)

type fakeSignedURLClient struct {
	url string
	Err error
}

func (f *fakeSignedURLClient) SignResumableUploadURL(_ context.Context, _ string, _ string, _ string) (string, error) {
	if f.Err != nil {
		return "", f.Err
	}
	return f.url, nil
}

type fakeUploadSessionStore struct {
	createdSessions []*model.UploadSession
	createErr       error
	byIdempotency   *model.UploadSession
	lookupErr       error
	getByIDSession  *model.UploadSession
	getByIDErr      error
	updatedStatus   *model.UploadStatus
	updatedBytes    *int64
	markedCompleted bool
	markedCancelled bool
	markedExpired   bool
}

func (f *fakeUploadSessionStore) Create(_ context.Context, session *model.UploadSession) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.createdSessions = append(f.createdSessions, session)
	return nil
}

func (f *fakeUploadSessionStore) GetByID(_ context.Context, _ string) (*model.UploadSession, error) {
	if f.getByIDErr != nil {
		return nil, f.getByIDErr
	}
	return f.getByIDSession, nil
}

func (f *fakeUploadSessionStore) GetByIdempotencyKey(_ context.Context, _, _, _ string) (*model.UploadSession, error) {
	if f.lookupErr != nil {
		return nil, f.lookupErr
	}
	return f.byIdempotency, nil
}

func (f *fakeUploadSessionStore) UpdateStatus(_ context.Context, _ string, status model.UploadStatus, uploadedBytes int64) error {
	f.updatedStatus = &status
	f.updatedBytes = &uploadedBytes
	return nil
}

func (f *fakeUploadSessionStore) UpdateGCSUploadURL(_ context.Context, _ string, _ string) error {
	return nil
}

func (f *fakeUploadSessionStore) MarkCompleted(_ context.Context, _ string, uploadedBytes int64) error {
	f.markedCompleted = true
	f.updatedBytes = &uploadedBytes
	return nil
}

func (f *fakeUploadSessionStore) MarkCancelled(_ context.Context, _ string) error {
	f.markedCancelled = true
	return nil
}

func (f *fakeUploadSessionStore) MarkExpired(_ context.Context, _ string) error {
	f.markedExpired = true
	return nil
}

func TestCreateUploadSession_InvalidInput(t *testing.T) {
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket"}, &fakeUploadSessionStore{}, otel.Tracer("test"))
	_, err := service.CreateUploadSession(context.Background(), dto.CreateUploadRequest{})
	if !errors.Is(err, internalerrors.ErrInvalidInput) {
		t.Fatalf("expected invalid input error")
	}
}

func TestCreateUploadSession_MissingBucket(t *testing.T) {
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{}, &fakeUploadSessionStore{}, otel.Tracer("test"))
	_, err := service.CreateUploadSession(context.Background(), dto.CreateUploadRequest{FileName: "file", ContentType: "video/mp4", SizeBytes: 1})
	if err == nil {
		t.Fatalf("expected missing bucket error")
	}
}

func TestCreateUploadSession_MissingStore(t *testing.T) {
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket"}, nil, otel.Tracer("test"))
	_, err := service.CreateUploadSession(context.Background(), dto.CreateUploadRequest{FileName: "file", ContentType: "video/mp4", SizeBytes: 1})
	if err == nil {
		t.Fatalf("expected missing store error")
	}
}

func TestCreateUploadSession_IdempotencyHit(t *testing.T) {
	store := &fakeUploadSessionStore{
		byIdempotency: &model.UploadSession{
			UploadID:     "upl_existing",
			GCSUploadURL: "existing-url",
			Bucket:       "bucket",
			ObjectName:   "uploads/upl_existing/file",
			Status:       model.StatusCreated,
			ExpiresAt:    time.Now().Add(10 * time.Minute),
		},
	}
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket", SERVICE_ACCOUNT_EMAIL: "sa"}, store, otel.Tracer("test"))

	resp, err := service.CreateUploadSession(context.Background(), dto.CreateUploadRequest{FileName: "file", ContentType: "video/mp4", SizeBytes: 1, IdempotencyKey: "idemp"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.UploadID != "upl_existing" {
		t.Fatalf("expected existing session to be reused")
	}
}

func TestCreateUploadSession_IdempotencyLookupError(t *testing.T) {
	store := &fakeUploadSessionStore{lookupErr: errors.New("lookup failed")}
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket", SERVICE_ACCOUNT_EMAIL: "sa"}, store, otel.Tracer("test"))

	_, err := service.CreateUploadSession(context.Background(), dto.CreateUploadRequest{FileName: "file", ContentType: "video/mp4", SizeBytes: 1, IdempotencyKey: "idemp"})
	if err == nil {
		t.Fatalf("expected lookup error")
	}
}

func TestCreateUploadSession_SignURLFailure(t *testing.T) {
	store := &fakeUploadSessionStore{}
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{Err: errors.New("sign failed")}, &config.Config{GCSBucket: "bucket", SERVICE_ACCOUNT_EMAIL: "sa"}, store, otel.Tracer("test"))

	_, err := service.CreateUploadSession(context.Background(), dto.CreateUploadRequest{FileName: "file", ContentType: "video/mp4", SizeBytes: 1})
	if err == nil {
		t.Fatalf("expected signing error")
	}
}

func TestCreateUploadSession_PersistFailure(t *testing.T) {
	store := &fakeUploadSessionStore{createErr: errors.New("db error")}
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket", SERVICE_ACCOUNT_EMAIL: "sa"}, store, otel.Tracer("test"))

	_, err := service.CreateUploadSession(context.Background(), dto.CreateUploadRequest{FileName: "file", ContentType: "video/mp4", SizeBytes: 1})
	if err == nil {
		t.Fatalf("expected persistence error")
	}
}

func TestCreateUploadSession_Success(t *testing.T) {
	store := &fakeUploadSessionStore{}
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket", SERVICE_ACCOUNT_EMAIL: "sa"}, store, otel.Tracer("test"))

	resp, err := service.CreateUploadSession(context.Background(), dto.CreateUploadRequest{
		FileName:       "file.mp4",
		ContentType:    "video/mp4",
		SizeBytes:      10,
		IdempotencyKey: "key",
		Metadata:       map[string]string{"k": "v"},
		Labels:         map[string]string{"type": "video"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GCSUploadURL != "signed" {
		t.Fatalf("expected signed url to be set")
	}
	if len(store.createdSessions) != 1 {
		t.Fatalf("expected session to be persisted")
	}

	session := store.createdSessions[0]
	if session.IdempotencyKey != "key" || session.Metadata["k"] != "v" || session.Labels["type"] != "video" {
		t.Fatalf("expected metadata and labels to be stored")
	}
}

func TestResumeUploadSession_InvalidInput(t *testing.T) {
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket"}, &fakeUploadSessionStore{}, otel.Tracer("test"))
	_, err := service.ResumeUploadSession(context.Background(), "")
	if !errors.Is(err, internalerrors.ErrInvalidInput) {
		t.Fatalf("expected invalid input error")
	}
}

func TestResumeUploadSession_MissingStore(t *testing.T) {
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket"}, nil, otel.Tracer("test"))
	_, err := service.ResumeUploadSession(context.Background(), "upl_1")
	if err == nil {
		t.Fatalf("expected missing store error")
	}
}

func TestResumeUploadSession_NotFound(t *testing.T) {
	store := &fakeUploadSessionStore{}
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket"}, store, otel.Tracer("test"))
	_, err := service.ResumeUploadSession(context.Background(), "upl_1")
	if !errors.Is(err, internalerrors.ErrNotFound) {
		t.Fatalf("expected not found error")
	}
}

func TestResumeUploadSession_Expired(t *testing.T) {
	store := &fakeUploadSessionStore{
		getByIDSession: &model.UploadSession{
			UploadID:  "upl_1",
			Status:    model.StatusCreated,
			ExpiresAt: time.Now().Add(-1 * time.Minute),
		},
	}
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket"}, store, otel.Tracer("test"))
	_, err := service.ResumeUploadSession(context.Background(), "upl_1")
	if !errors.Is(err, internalerrors.ErrSessionExpired) {
		t.Fatalf("expected expired error")
	}
}

func TestResumeUploadSession_Success(t *testing.T) {
	store := &fakeUploadSessionStore{
		getByIDSession: &model.UploadSession{
			UploadID:     "upl_1",
			GCSUploadURL: "signed",
			Status:       model.StatusCreated,
			ExpiresAt:    time.Now().Add(10 * time.Minute),
		},
	}
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket"}, store, otel.Tracer("test"))
	resp, err := service.ResumeUploadSession(context.Background(), "upl_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.UploadID != "upl_1" || resp.GCSUploadURL != "signed" {
		t.Fatalf("expected resume response to be populated")
	}
}

func TestGetUploadStatus_InvalidInput(t *testing.T) {
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket"}, &fakeUploadSessionStore{}, otel.Tracer("test"))
	_, err := service.GetUploadStatus(context.Background(), "")
	if !errors.Is(err, internalerrors.ErrInvalidInput) {
		t.Fatalf("expected invalid input error")
	}
}

func TestGetUploadStatus_MissingStore(t *testing.T) {
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket"}, nil, otel.Tracer("test"))
	_, err := service.GetUploadStatus(context.Background(), "upl_1")
	if err == nil {
		t.Fatalf("expected missing store error")
	}
}

func TestGetUploadStatus_NotFound(t *testing.T) {
	store := &fakeUploadSessionStore{}
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket"}, store, otel.Tracer("test"))
	_, err := service.GetUploadStatus(context.Background(), "upl_1")
	if !errors.Is(err, internalerrors.ErrNotFound) {
		t.Fatalf("expected not found error")
	}
}

func TestGetUploadStatus_Expired(t *testing.T) {
	store := &fakeUploadSessionStore{
		getByIDSession: &model.UploadSession{
			UploadID:  "upl_1",
			Status:    model.StatusCreated,
			ExpiresAt: time.Now().Add(-1 * time.Minute),
		},
	}
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket"}, store, otel.Tracer("test"))
	_, err := service.GetUploadStatus(context.Background(), "upl_1")
	if !errors.Is(err, internalerrors.ErrSessionExpired) {
		t.Fatalf("expected expired error")
	}
}

func TestGetUploadStatus_Success(t *testing.T) {
	store := &fakeUploadSessionStore{
		getByIDSession: &model.UploadSession{
			UploadID:      "upl_1",
			GCSUploadURL:  "signed",
			Status:        model.StatusInProgress,
			ExpiresAt:     time.Now().Add(10 * time.Minute),
			SizeBytes:     42,
			ContentType:   "video/mp4",
			ObjectName:    "uploads/upl_1/file.mp4",
			CreatedAt:     time.Now().Add(-5 * time.Minute),
			UpdatedAt:     time.Now(),
			UploadedBytes: 10,
		},
	}
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket"}, store, otel.Tracer("test"))
	resp, err := service.GetUploadStatus(context.Background(), "upl_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.UploadID != "upl_1" || resp.Status != dto.UploadStatus(model.StatusInProgress) {
		t.Fatalf("expected status response to be populated")
	}
}

func TestQueryUploadStatus_InvalidInput(t *testing.T) {
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket"}, &fakeUploadSessionStore{}, otel.Tracer("test"))
	_, err := service.QueryUploadStatus(context.Background(), "", dto.QueryStatusRequest{})
	if !errors.Is(err, internalerrors.ErrInvalidInput) {
		t.Fatalf("expected invalid input error")
	}
}

func TestQueryUploadStatus_MissingStore(t *testing.T) {
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket"}, nil, otel.Tracer("test"))
	_, err := service.QueryUploadStatus(context.Background(), "upl_1", dto.QueryStatusRequest{})
	if err == nil {
		t.Fatalf("expected missing store error")
	}
}

func TestQueryUploadStatus_NotFound(t *testing.T) {
	store := &fakeUploadSessionStore{}
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket"}, store, otel.Tracer("test"))
	_, err := service.QueryUploadStatus(context.Background(), "upl_1", dto.QueryStatusRequest{})
	if !errors.Is(err, internalerrors.ErrNotFound) {
		t.Fatalf("expected not found error")
	}
}

func TestQueryUploadStatus_Expired(t *testing.T) {
	store := &fakeUploadSessionStore{
		getByIDSession: &model.UploadSession{
			UploadID:  "upl_1",
			Status:    model.StatusCreated,
			ExpiresAt: time.Now().Add(-1 * time.Minute),
		},
	}
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket"}, store, otel.Tracer("test"))
	_, err := service.QueryUploadStatus(context.Background(), "upl_1", dto.QueryStatusRequest{})
	if !errors.Is(err, internalerrors.ErrSessionExpired) {
		t.Fatalf("expected expired error")
	}
}

func TestQueryUploadStatus_Success(t *testing.T) {
	store := &fakeUploadSessionStore{
		getByIDSession: &model.UploadSession{
			UploadID:      "upl_1",
			Status:        model.StatusInProgress,
			ExpiresAt:     time.Now().Add(10 * time.Minute),
			UploadedBytes: 123,
			SizeBytes:     1000,
		},
	}
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket"}, store, otel.Tracer("test"))
	resp, err := service.QueryUploadStatus(context.Background(), "upl_1", dto.QueryStatusRequest{Refresh: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.UploadedBytes != 123 {
		t.Fatalf("expected uploaded bytes to be returned")
	}
}

func TestQueryUploadStatus_RefreshUpdatesBytes(t *testing.T) {
	store := &fakeUploadSessionStore{
		getByIDSession: &model.UploadSession{
			UploadID:     "upl_1",
			Status:       model.StatusInProgress,
			ExpiresAt:    time.Now().Add(10 * time.Minute),
			SizeBytes:    1000,
			GCSUploadURL: "http://example.com/upload",
		},
	}
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket"}, store, otel.Tracer("test"))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Range", "bytes=0-499")
		w.WriteHeader(308)
	}))
	defer server.Close()
	store.getByIDSession.GCSUploadURL = server.URL

	resp, err := service.QueryUploadStatus(context.Background(), "upl_1", dto.QueryStatusRequest{Refresh: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.UploadedBytes != 500 {
		t.Fatalf("expected uploaded bytes to be refreshed")
	}
	if store.updatedBytes == nil || *store.updatedBytes != 500 {
		t.Fatalf("expected store to be updated")
	}
}

func TestQueryUploadStatus_RefreshCompletes(t *testing.T) {
	store := &fakeUploadSessionStore{
		getByIDSession: &model.UploadSession{
			UploadID:     "upl_1",
			Status:       model.StatusInProgress,
			ExpiresAt:    time.Now().Add(10 * time.Minute),
			SizeBytes:    100,
			GCSUploadURL: "http://example.com/upload",
		},
	}
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket"}, store, otel.Tracer("test"))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	store.getByIDSession.GCSUploadURL = server.URL

	resp, err := service.QueryUploadStatus(context.Background(), "upl_1", dto.QueryStatusRequest{Refresh: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != dto.UploadStatus(model.StatusCompleted) {
		t.Fatalf("expected status to be completed")
	}
	if !store.markedCompleted {
		t.Fatalf("expected store to mark completed")
	}
}

func TestCancelUploadSession_InvalidInput(t *testing.T) {
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket"}, &fakeUploadSessionStore{}, otel.Tracer("test"))
	_, err := service.CancelUploadSession(context.Background(), "", dto.CancelUploadRequest{})
	if !errors.Is(err, internalerrors.ErrInvalidInput) {
		t.Fatalf("expected invalid input error")
	}
}

func TestCancelUploadSession_MissingStore(t *testing.T) {
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket"}, nil, otel.Tracer("test"))
	_, err := service.CancelUploadSession(context.Background(), "upl_1", dto.CancelUploadRequest{})
	if err == nil {
		t.Fatalf("expected missing store error")
	}
}

func TestCancelUploadSession_NotFound(t *testing.T) {
	store := &fakeUploadSessionStore{}
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket"}, store, otel.Tracer("test"))
	_, err := service.CancelUploadSession(context.Background(), "upl_1", dto.CancelUploadRequest{})
	if !errors.Is(err, internalerrors.ErrNotFound) {
		t.Fatalf("expected not found error")
	}
}

func TestCancelUploadSession_Expired(t *testing.T) {
	store := &fakeUploadSessionStore{
		getByIDSession: &model.UploadSession{
			UploadID:  "upl_1",
			Status:    model.StatusCreated,
			ExpiresAt: time.Now().Add(-1 * time.Minute),
		},
	}
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket"}, store, otel.Tracer("test"))
	_, err := service.CancelUploadSession(context.Background(), "upl_1", dto.CancelUploadRequest{})
	if !errors.Is(err, internalerrors.ErrSessionExpired) {
		t.Fatalf("expected expired error")
	}
}

func TestCancelUploadSession_Success(t *testing.T) {
	store := &fakeUploadSessionStore{
		getByIDSession: &model.UploadSession{
			UploadID:  "upl_1",
			Status:    model.StatusInProgress,
			ExpiresAt: time.Now().Add(10 * time.Minute),
		},
	}
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket"}, store, otel.Tracer("test"))
	resp, err := service.CancelUploadSession(context.Background(), "upl_1", dto.CancelUploadRequest{Reason: "user_cancelled"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != dto.StatusCancelled {
		t.Fatalf("expected cancelled status")
	}
}

// --- Additional coverage for ResumeUploadSession store error ---

func TestResumeUploadSession_StoreError(t *testing.T) {
	store := &fakeUploadSessionStore{getByIDErr: errors.New("db failure")}
	svc := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket"}, store, otel.Tracer("test"))
	_, err := svc.ResumeUploadSession(context.Background(), "upl_1")
	if err == nil {
		t.Fatal("expected store error")
	}
}

// --- Additional coverage for GetUploadStatus store error ---

func TestGetUploadStatus_StoreError(t *testing.T) {
	store := &fakeUploadSessionStore{getByIDErr: errors.New("db failure")}
	svc := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket"}, store, otel.Tracer("test"))
	_, err := svc.GetUploadStatus(context.Background(), "upl_1")
	if err == nil {
		t.Fatal("expected store error")
	}
}

// --- Additional coverage for QueryUploadStatus store error ---

func TestQueryUploadStatus_StoreError(t *testing.T) {
	store := &fakeUploadSessionStore{getByIDErr: errors.New("db failure")}
	svc := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket"}, store, otel.Tracer("test"))
	_, err := svc.QueryUploadStatus(context.Background(), "upl_1", dto.QueryStatusRequest{})
	if err == nil {
		t.Fatal("expected store error")
	}
}

// --- Additional coverage for CancelUploadSession store errors ---

func TestCancelUploadSession_StoreError(t *testing.T) {
	store := &fakeUploadSessionStore{getByIDErr: errors.New("db failure")}
	svc := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket"}, store, otel.Tracer("test"))
	_, err := svc.CancelUploadSession(context.Background(), "upl_1", dto.CancelUploadRequest{})
	if err == nil {
		t.Fatal("expected store error")
	}
}

func TestCancelUploadSession_MarkCancelledError(t *testing.T) {
	store := &fakeUploadSessionStoreWithCancelErr{
		fakeUploadSessionStore: fakeUploadSessionStore{
			getByIDSession: &model.UploadSession{
				UploadID:  "upl_1",
				Status:    model.StatusInProgress,
				ExpiresAt: time.Now().Add(10 * time.Minute),
			},
		},
		cancelErr: errors.New("cancel failed"),
	}
	svc := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket"}, store, otel.Tracer("test"))
	_, err := svc.CancelUploadSession(context.Background(), "upl_1", dto.CancelUploadRequest{})
	if err == nil {
		t.Fatal("expected cancel error")
	}
}

// fakeUploadSessionStoreWithCancelErr wraps fakeUploadSessionStore to return errors from MarkCancelled.
type fakeUploadSessionStoreWithCancelErr struct {
	fakeUploadSessionStore
	cancelErr error
}

func (f *fakeUploadSessionStoreWithCancelErr) MarkCancelled(_ context.Context, _ string) error {
	return f.cancelErr
}

// --- Additional coverage for QueryUploadStatus with GCS error path ---

func TestQueryUploadStatus_RefreshGCSError(t *testing.T) {
	store := &fakeUploadSessionStore{
		getByIDSession: &model.UploadSession{
			UploadID:     "upl_1",
			Status:       model.StatusInProgress,
			ExpiresAt:    time.Now().Add(10 * time.Minute),
			SizeBytes:    1000,
			GCSUploadURL: "http://localhost:1/bad-url", // unreachable
		},
	}
	svc := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket"}, store, otel.Tracer("test"))
	_, err := svc.QueryUploadStatus(context.Background(), "upl_1", dto.QueryStatusRequest{Refresh: true})
	if err == nil {
		t.Fatal("expected GCS query error")
	}
}

func TestQueryUploadStatus_RefreshMarkCompletedError(t *testing.T) {
	store := &fakeUploadSessionStoreWithCompletedErr{
		fakeUploadSessionStore: fakeUploadSessionStore{
			getByIDSession: &model.UploadSession{
				UploadID:     "upl_1",
				Status:       model.StatusInProgress,
				ExpiresAt:    time.Now().Add(10 * time.Minute),
				SizeBytes:    100,
				GCSUploadURL: "placeholder",
			},
		},
		completedErr: errors.New("mark completed failed"),
	}
	// Set up test server that returns HTTP 200 (upload complete)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	store.getByIDSession.GCSUploadURL = server.URL

	svc := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket"}, store, otel.Tracer("test"))
	_, err := svc.QueryUploadStatus(context.Background(), "upl_1", dto.QueryStatusRequest{Refresh: true})
	if err == nil {
		t.Fatal("expected mark completed error")
	}
}

type fakeUploadSessionStoreWithCompletedErr struct {
	fakeUploadSessionStore
	completedErr error
}

func (f *fakeUploadSessionStoreWithCompletedErr) MarkCompleted(_ context.Context, _ string, _ int64) error {
	return f.completedErr
}

func TestQueryUploadStatus_RefreshUpdateStatusError(t *testing.T) {
	store := &fakeUploadSessionStoreWithUpdateErr{
		fakeUploadSessionStore: fakeUploadSessionStore{
			getByIDSession: &model.UploadSession{
				UploadID:     "upl_1",
				Status:       model.StatusInProgress,
				ExpiresAt:    time.Now().Add(10 * time.Minute),
				SizeBytes:    1000,
				GCSUploadURL: "placeholder",
			},
		},
		updateErr: errors.New("update status failed"),
	}
	// Set up test server that returns 308 with partial range
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Range", "bytes=0-499")
		w.WriteHeader(308)
	}))
	defer server.Close()
	store.getByIDSession.GCSUploadURL = server.URL

	svc := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket"}, store, otel.Tracer("test"))
	_, err := svc.QueryUploadStatus(context.Background(), "upl_1", dto.QueryStatusRequest{Refresh: true})
	if err == nil {
		t.Fatal("expected update status error")
	}
}

type fakeUploadSessionStoreWithUpdateErr struct {
	fakeUploadSessionStore
	updateErr error
}

func (f *fakeUploadSessionStoreWithUpdateErr) UpdateStatus(_ context.Context, _ string, _ model.UploadStatus, _ int64) error {
	return f.updateErr
}

// --- Helper function tests ---

func TestBuildObjectName(t *testing.T) {
	tests := []struct {
		name     string
		uploadID string
		fileName string
		want     string
	}{
		{"with file name", "upl_1", "file.mp4", "uploads/upl_1/file.mp4"},
		{"empty file name", "upl_1", "", "uploads/upl_1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildObjectName(tt.uploadID, tt.fileName)
			if got != tt.want {
				t.Fatalf("expected %s, got %s", tt.want, got)
			}
		})
	}
}

func TestChecksumAlgorithm(t *testing.T) {
	if checksumAlgorithm(nil) != "" {
		t.Fatal("expected empty string for nil checksum")
	}
	if checksumAlgorithm(&dto.ChecksumRequest{Algorithm: "md5"}) != "md5" {
		t.Fatal("expected md5")
	}
}

func TestChecksumValue(t *testing.T) {
	if checksumValue(nil) != "" {
		t.Fatal("expected empty string for nil checksum")
	}
	if checksumValue(&dto.ChecksumRequest{Value: "abc"}) != "abc" {
		t.Fatal("expected abc")
	}
}

func TestGenerateUploadID(t *testing.T) {
	id := generateUploadID()
	if id == "" {
		t.Fatal("expected non-empty upload ID")
	}
	if len(id) < 4 || id[:4] != "upl_" {
		t.Fatalf("expected prefix 'upl_', got %s", id)
	}
	// Verify uniqueness
	id2 := generateUploadID()
	if id == id2 {
		t.Fatal("expected unique IDs")
	}
}

func TestParseUploadedBytes(t *testing.T) {
	tests := []struct {
		name    string
		header  string
		want    int64
		wantErr bool
	}{
		{"empty header", "", 0, false},
		{"whitespace only", "   ", 0, false},
		{"valid range", "bytes=0-499", 500, false},
		{"with prefix", "bytes=0-99", 100, false},
		{"invalid format", "invalid", 0, true},
		{"invalid end value", "bytes=0-abc", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseUploadedBytes(tt.header)
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected %d, got %d", tt.want, got)
			}
		})
	}
}

func TestQueryUploadedBytes_MissingURL(t *testing.T) {
	_, err := queryUploadedBytes(context.Background(), "", 100)
	if err == nil {
		t.Fatal("expected error for missing URL")
	}
}

func TestQueryUploadedBytes_InvalidSizeBytes(t *testing.T) {
	_, err := queryUploadedBytes(context.Background(), "http://example.com", 0)
	if err == nil {
		t.Fatal("expected error for invalid size bytes")
	}
}

func TestQueryUploadedBytes_Completed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	bytes, err := queryUploadedBytes(context.Background(), server.URL, 1000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bytes != 1000 {
		t.Fatalf("expected 1000, got %d", bytes)
	}
}

func TestQueryUploadedBytes_Created(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	bytes, err := queryUploadedBytes(context.Background(), server.URL, 1000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bytes != 1000 {
		t.Fatalf("expected 1000, got %d", bytes)
	}
}

func TestQueryUploadedBytes_NoContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	bytes, err := queryUploadedBytes(context.Background(), server.URL, 1000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bytes != 1000 {
		t.Fatalf("expected 1000, got %d", bytes)
	}
}

func TestQueryUploadedBytes_InProgress(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Range", "bytes=0-499")
		w.WriteHeader(308)
	}))
	defer server.Close()

	bytes, err := queryUploadedBytes(context.Background(), server.URL, 1000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bytes != 500 {
		t.Fatalf("expected 500, got %d", bytes)
	}
}

func TestQueryUploadedBytes_InProgress_NoRange(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(308)
	}))
	defer server.Close()

	bytes, err := queryUploadedBytes(context.Background(), server.URL, 1000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bytes != 0 {
		t.Fatalf("expected 0 for missing Range header, got %d", bytes)
	}
}

func TestQueryUploadedBytes_UnexpectedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	_, err := queryUploadedBytes(context.Background(), server.URL, 1000)
	if err == nil {
		t.Fatal("expected error for unexpected status")
	}
}

func TestQueryUploadStatus_NoRefresh(t *testing.T) {
	store := &fakeUploadSessionStore{
		getByIDSession: &model.UploadSession{
			UploadID:      "upl_1",
			Status:        model.StatusInProgress,
			ExpiresAt:     time.Now().Add(10 * time.Minute),
			UploadedBytes: 42,
			SizeBytes:     100,
		},
	}
	svc := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket"}, store, otel.Tracer("test"))
	resp, err := svc.QueryUploadStatus(context.Background(), "upl_1", dto.QueryStatusRequest{Refresh: false})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.UploadedBytes != 42 {
		t.Fatalf("expected 42, got %d", resp.UploadedBytes)
	}
}

func TestQueryUploadStatus_RefreshEmptyGCSURL(t *testing.T) {
	store := &fakeUploadSessionStore{
		getByIDSession: &model.UploadSession{
			UploadID:      "upl_1",
			Status:        model.StatusInProgress,
			ExpiresAt:     time.Now().Add(10 * time.Minute),
			UploadedBytes: 42,
			SizeBytes:     100,
			GCSUploadURL:  "", // empty URL
		},
	}
	svc := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket"}, store, otel.Tracer("test"))
	resp, err := svc.QueryUploadStatus(context.Background(), "upl_1", dto.QueryStatusRequest{Refresh: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.UploadedBytes != 42 {
		t.Fatalf("expected 42 (unchanged), got %d", resp.UploadedBytes)
	}
}

func TestMapSessionToCreateResponse(t *testing.T) {
	now := time.Now().UTC()
	session := &model.UploadSession{
		UploadID:     "upl_1",
		GCSUploadURL: "https://storage.googleapis.com/upload",
		Bucket:       "my-bucket",
		ObjectName:   "uploads/upl_1/file.pdf",
		Status:       model.StatusCreated,
		ExpiresAt:    now,
	}
	resp := mapSessionToCreateResponse(session)
	if resp.UploadID != "upl_1" {
		t.Fatalf("expected upl_1, got %s", resp.UploadID)
	}
	if resp.GCSUploadURL != "https://storage.googleapis.com/upload" {
		t.Fatalf("unexpected GCS URL")
	}
	if resp.Bucket != "my-bucket" {
		t.Fatalf("unexpected bucket")
	}
	if resp.ObjectName != "uploads/upl_1/file.pdf" {
		t.Fatalf("unexpected object name")
	}
	if resp.Status != dto.UploadStatus(model.StatusCreated) {
		t.Fatalf("unexpected status")
	}
	if resp.SessionExpiresAt != now.Format(time.RFC3339) {
		t.Fatalf("unexpected expiry time")
	}
}

// TestQueryUploadedBytes_BadURL verifies that queryUploadedBytes returns an
// error when the URL causes http.NewRequestWithContext to fail (e.g., URL
// containing a control character).
func TestQueryUploadedBytes_BadURL(t *testing.T) {
	// A URL containing a null byte causes NewRequestWithContext to fail.
	_, err := queryUploadedBytes(context.Background(), "http://example.com/\x00bad", 100)
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}
