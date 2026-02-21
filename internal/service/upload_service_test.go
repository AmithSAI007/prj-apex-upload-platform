package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/AmithSAI007/prj-apex-upload-platform/internal/api/dto"
	"github.com/AmithSAI007/prj-apex-upload-platform/internal/config"
	internalerrors "github.com/AmithSAI007/prj-apex-upload-platform/internal/errors"
	"github.com/AmithSAI007/prj-apex-upload-platform/internal/model"
	"go.uber.org/zap"
)

type fakeSignedURLClient struct {
	url string
	Err error
}

func (f *fakeSignedURLClient) SignResumableUploadURL(_ string, _ string, _ string) (string, error) {
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
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket"}, &fakeUploadSessionStore{})
	_, err := service.CreateUploadSession(context.Background(), dto.CreateUploadRequest{})
	if !errors.Is(err, internalerrors.ErrInvalidInput) {
		t.Fatalf("expected invalid input error")
	}
}

func TestCreateUploadSession_MissingBucket(t *testing.T) {
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{}, &fakeUploadSessionStore{})
	_, err := service.CreateUploadSession(context.Background(), dto.CreateUploadRequest{FileName: "file", ContentType: "video/mp4", SizeBytes: 1})
	if err == nil {
		t.Fatalf("expected missing bucket error")
	}
}

func TestCreateUploadSession_MissingStore(t *testing.T) {
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket"}, nil)
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
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket", SERVICE_ACCOUNT_EMAIL: "sa"}, store)

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
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket", SERVICE_ACCOUNT_EMAIL: "sa"}, store)

	_, err := service.CreateUploadSession(context.Background(), dto.CreateUploadRequest{FileName: "file", ContentType: "video/mp4", SizeBytes: 1, IdempotencyKey: "idemp"})
	if err == nil {
		t.Fatalf("expected lookup error")
	}
}

func TestCreateUploadSession_SignURLFailure(t *testing.T) {
	store := &fakeUploadSessionStore{}
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{Err: errors.New("sign failed")}, &config.Config{GCSBucket: "bucket", SERVICE_ACCOUNT_EMAIL: "sa"}, store)

	_, err := service.CreateUploadSession(context.Background(), dto.CreateUploadRequest{FileName: "file", ContentType: "video/mp4", SizeBytes: 1})
	if err == nil {
		t.Fatalf("expected signing error")
	}
}

func TestCreateUploadSession_PersistFailure(t *testing.T) {
	store := &fakeUploadSessionStore{createErr: errors.New("db error")}
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket", SERVICE_ACCOUNT_EMAIL: "sa"}, store)

	_, err := service.CreateUploadSession(context.Background(), dto.CreateUploadRequest{FileName: "file", ContentType: "video/mp4", SizeBytes: 1})
	if err == nil {
		t.Fatalf("expected persistence error")
	}
}

func TestCreateUploadSession_Success(t *testing.T) {
	store := &fakeUploadSessionStore{}
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket", SERVICE_ACCOUNT_EMAIL: "sa"}, store)

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
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket"}, &fakeUploadSessionStore{})
	_, err := service.ResumeUploadSession(context.Background(), "")
	if !errors.Is(err, internalerrors.ErrInvalidInput) {
		t.Fatalf("expected invalid input error")
	}
}

func TestResumeUploadSession_MissingStore(t *testing.T) {
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket"}, nil)
	_, err := service.ResumeUploadSession(context.Background(), "upl_1")
	if err == nil {
		t.Fatalf("expected missing store error")
	}
}

func TestResumeUploadSession_NotFound(t *testing.T) {
	store := &fakeUploadSessionStore{}
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket"}, store)
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
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket"}, store)
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
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket"}, store)
	resp, err := service.ResumeUploadSession(context.Background(), "upl_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.UploadID != "upl_1" || resp.GCSUploadURL != "signed" {
		t.Fatalf("expected resume response to be populated")
	}
}

func TestGetUploadStatus_InvalidInput(t *testing.T) {
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket"}, &fakeUploadSessionStore{})
	_, err := service.GetUploadStatus(context.Background(), "")
	if !errors.Is(err, internalerrors.ErrInvalidInput) {
		t.Fatalf("expected invalid input error")
	}
}

func TestGetUploadStatus_MissingStore(t *testing.T) {
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket"}, nil)
	_, err := service.GetUploadStatus(context.Background(), "upl_1")
	if err == nil {
		t.Fatalf("expected missing store error")
	}
}

func TestGetUploadStatus_NotFound(t *testing.T) {
	store := &fakeUploadSessionStore{}
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket"}, store)
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
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket"}, store)
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
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket"}, store)
	resp, err := service.GetUploadStatus(context.Background(), "upl_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.UploadID != "upl_1" || resp.Status != dto.UploadStatus(model.StatusInProgress) {
		t.Fatalf("expected status response to be populated")
	}
}

func TestQueryUploadStatus_InvalidInput(t *testing.T) {
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket"}, &fakeUploadSessionStore{})
	_, err := service.QueryUploadStatus(context.Background(), "", dto.QueryStatusRequest{})
	if !errors.Is(err, internalerrors.ErrInvalidInput) {
		t.Fatalf("expected invalid input error")
	}
}

func TestQueryUploadStatus_MissingStore(t *testing.T) {
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket"}, nil)
	_, err := service.QueryUploadStatus(context.Background(), "upl_1", dto.QueryStatusRequest{})
	if err == nil {
		t.Fatalf("expected missing store error")
	}
}

func TestQueryUploadStatus_NotFound(t *testing.T) {
	store := &fakeUploadSessionStore{}
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket"}, store)
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
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket"}, store)
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
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket"}, store)
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
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket"}, store)

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
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket"}, store)

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
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket"}, &fakeUploadSessionStore{})
	_, err := service.CancelUploadSession(context.Background(), "", dto.CancelUploadRequest{})
	if !errors.Is(err, internalerrors.ErrInvalidInput) {
		t.Fatalf("expected invalid input error")
	}
}

func TestCancelUploadSession_MissingStore(t *testing.T) {
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket"}, nil)
	_, err := service.CancelUploadSession(context.Background(), "upl_1", dto.CancelUploadRequest{})
	if err == nil {
		t.Fatalf("expected missing store error")
	}
}

func TestCancelUploadSession_NotFound(t *testing.T) {
	store := &fakeUploadSessionStore{}
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket"}, store)
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
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket"}, store)
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
	service := NewUploadService(zap.NewNop(), &fakeSignedURLClient{url: "signed"}, &config.Config{GCSBucket: "bucket"}, store)
	resp, err := service.CancelUploadSession(context.Background(), "upl_1", dto.CancelUploadRequest{Reason: "user_cancelled"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != dto.StatusCancelled {
		t.Fatalf("expected cancelled status")
	}
}
