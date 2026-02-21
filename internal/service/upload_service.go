package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/AmithSAI007/prj-apex-upload-platform/api/middleware"
	"github.com/AmithSAI007/prj-apex-upload-platform/internal/api/dto"
	"github.com/AmithSAI007/prj-apex-upload-platform/internal/config"
	internalerrors "github.com/AmithSAI007/prj-apex-upload-platform/internal/errors"
	"github.com/AmithSAI007/prj-apex-upload-platform/internal/model"
	"github.com/AmithSAI007/prj-apex-upload-platform/internal/storage"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// UploadInterface defines upload session business operations.
type UploadInterface interface {
	CreateUploadSession(ctx context.Context, req dto.CreateUploadRequest) (dto.CreateUploadResponse, error)
	ResumeUploadSession(ctx context.Context, uploadID string) (dto.ResumeUploadResponse, error)
	GetUploadStatus(ctx context.Context, uploadID string) (dto.UploadStatusResponse, error)
	QueryUploadStatus(ctx context.Context, uploadID string, req dto.QueryStatusRequest) (dto.QueryStatusResponse, error)
	CancelUploadSession(ctx context.Context, uploadID string, req dto.CancelUploadRequest) (dto.CancelUploadResponse, error)
}

// UploadService orchestrates upload session creation and persistence.
type UploadService struct {
	logger *zap.Logger
	gcs    storage.SignedURLClient
	cfg    *config.Config
	store  UploadSessionStore
}

// NewUploadService constructs the upload service with dependencies.
func NewUploadService(logger *zap.Logger, gcsClient storage.SignedURLClient, cfg *config.Config, store UploadSessionStore) *UploadService {
	return &UploadService{
		logger: logger,
		gcs:    gcsClient,
		cfg:    cfg,
		store:  store,
	}
}

// CreateUploadSession creates a new resumable upload session and persists it.
func (s *UploadService) CreateUploadSession(ctx context.Context, req dto.CreateUploadRequest) (dto.CreateUploadResponse, error) {
	startTime := time.Now().UTC()
	if req.FileName == "" || req.ContentType == "" || req.SizeBytes <= 0 {
		return dto.CreateUploadResponse{}, internalerrors.ErrInvalidInput
	}

	bucketName := s.cfg.GCSBucket
	if bucketName == "" {
		return dto.CreateUploadResponse{}, errors.New("missing GCS signing configuration")
	}
	if s.store == nil {
		return dto.CreateUploadResponse{}, errors.New("missing upload session store")
	}

	if req.IdempotencyKey != "" {
		existing, err := s.store.GetByIdempotencyKey(ctx, "", "", req.IdempotencyKey)
		if err != nil {
			return dto.CreateUploadResponse{}, fmt.Errorf("lookup idempotency key: %w", err)
		}
		if existing != nil {
			s.logger.Info("Reusing upload session for idempotency key", zap.String("upload_id", existing.UploadID))
			return mapSessionToCreateResponse(existing), nil
		}
	}

	uploadID := generateUploadID()
	objectName := buildObjectName(uploadID, req.FileName)
	signedURL, err := s.gcs.SignResumableUploadURL(bucketName, objectName, s.cfg.SERVICE_ACCOUNT_EMAIL)
	if err != nil {
		return dto.CreateUploadResponse{}, fmt.Errorf("create signed resumable upload url: %w", err)
	}

	createdAt := startTime
	expiresAt := startTime.Add(defaultUploadTTL)
	session := &model.UploadSession{
		UploadID:       uploadID,
		TenantID:       "",
		UserID:         "",
		Bucket:         bucketName,
		ObjectName:     objectName,
		ContentType:    req.ContentType,
		SizeBytes:      req.SizeBytes,
		Status:         model.StatusCreated,
		GCSUploadURL:   signedURL,
		UploadedBytes:  0,
		ChecksumAlg:    checksumAlgorithm(req.Checksum),
		ChecksumValue:  checksumValue(req.Checksum),
		Metadata:       req.Metadata,
		Labels:         req.Labels,
		IdempotencyKey: req.IdempotencyKey,
		CreatedAt:      createdAt,
		UpdatedAt:      createdAt,
		ExpiresAt:      expiresAt,
	}

	if err := s.store.Create(ctx, session); err != nil {
		return dto.CreateUploadResponse{}, fmt.Errorf("persist upload session: %w", err)
	}

	response := mapSessionToCreateResponse(session)

	s.logger.Info(
		"Created upload session",
		zap.String("upload_id", uploadID),
		zap.String("object_name", objectName),
		zap.String("bucket", bucketName),
		zap.Int64("size_bytes", req.SizeBytes),
		zap.String("trace_id", middleware.TraceIDFromContext(ctx)),
	)
	return response, nil
}

func (s *UploadService) ResumeUploadSession(ctx context.Context, uploadID string) (dto.ResumeUploadResponse, error) {
	if uploadID == "" {
		return dto.ResumeUploadResponse{}, internalerrors.ErrInvalidInput
	}
	if s.store == nil {
		return dto.ResumeUploadResponse{}, errors.New("missing upload session store")
	}

	session, err := s.store.GetByID(ctx, uploadID)
	if err != nil {
		return dto.ResumeUploadResponse{}, fmt.Errorf("load upload session: %w", err)
	}
	if session == nil {
		return dto.ResumeUploadResponse{}, internalerrors.ErrNotFound
	}

	if time.Now().UTC().After(session.ExpiresAt) {
		_ = s.store.MarkExpired(ctx, uploadID)
		return dto.ResumeUploadResponse{}, internalerrors.ErrSessionExpired
	}

	response := dto.ResumeUploadResponse{
		UploadID:         session.UploadID,
		GCSUploadURL:     session.GCSUploadURL,
		SessionExpiresAt: session.ExpiresAt.Format(time.RFC3339),
		Status:           dto.UploadStatus(session.Status),
	}

	s.logger.Info(
		"Resumed upload session",
		zap.String("upload_id", uploadID),
		zap.String("trace_id", middleware.TraceIDFromContext(ctx)),
	)

	return response, nil
}

func (s *UploadService) GetUploadStatus(ctx context.Context, uploadID string) (dto.UploadStatusResponse, error) {
	if uploadID == "" {
		return dto.UploadStatusResponse{}, internalerrors.ErrInvalidInput
	}
	if s.store == nil {
		return dto.UploadStatusResponse{}, errors.New("missing upload session store")
	}

	session, err := s.store.GetByID(ctx, uploadID)
	if err != nil {
		return dto.UploadStatusResponse{}, fmt.Errorf("load upload session: %w", err)
	}
	if session == nil {
		return dto.UploadStatusResponse{}, internalerrors.ErrNotFound
	}

	if time.Now().UTC().After(session.ExpiresAt) {
		_ = s.store.MarkExpired(ctx, uploadID)
		return dto.UploadStatusResponse{}, internalerrors.ErrSessionExpired
	}

	response := dto.UploadStatusResponse{
		UploadID:      session.UploadID,
		Status:        dto.UploadStatus(session.Status),
		SizeBytes:     session.SizeBytes,
		ContentType:   session.ContentType,
		ObjectName:    session.ObjectName,
		CreatedAt:     session.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     session.UpdatedAt.Format(time.RFC3339),
		UploadedBytes: session.UploadedBytes,
	}

	s.logger.Info(
		"Fetched upload status",
		zap.String("upload_id", uploadID),
		zap.String("trace_id", middleware.TraceIDFromContext(ctx)),
	)

	return response, nil
}

func (s *UploadService) QueryUploadStatus(ctx context.Context, uploadID string, req dto.QueryStatusRequest) (dto.QueryStatusResponse, error) {
	if uploadID == "" {
		return dto.QueryStatusResponse{}, internalerrors.ErrInvalidInput
	}
	if s.store == nil {
		return dto.QueryStatusResponse{}, errors.New("missing upload session store")
	}

	session, err := s.store.GetByID(ctx, uploadID)
	if err != nil {
		return dto.QueryStatusResponse{}, fmt.Errorf("load upload session: %w", err)
	}
	if session == nil {
		return dto.QueryStatusResponse{}, internalerrors.ErrNotFound
	}

	if time.Now().UTC().After(session.ExpiresAt) {
		_ = s.store.MarkExpired(ctx, uploadID)
		return dto.QueryStatusResponse{}, internalerrors.ErrSessionExpired
	}

	if req.Refresh {
		uploadedBytes, err := queryUploadedBytes(ctx, session.GCSUploadURL, session.SizeBytes)
		if err != nil {
			return dto.QueryStatusResponse{}, fmt.Errorf("query gcs upload status: %w", err)
		}
		if uploadedBytes >= session.SizeBytes {
			if err := s.store.MarkCompleted(ctx, uploadID, session.SizeBytes); err != nil {
				return dto.QueryStatusResponse{}, fmt.Errorf("mark upload completed: %w", err)
			}
			session.Status = model.StatusCompleted
			session.UploadedBytes = session.SizeBytes
		} else {
			if err := s.store.UpdateStatus(ctx, uploadID, session.Status, uploadedBytes); err != nil {
				return dto.QueryStatusResponse{}, fmt.Errorf("update upload status: %w", err)
			}
			session.UploadedBytes = uploadedBytes
		}
	}

	response := dto.QueryStatusResponse{
		UploadID:      session.UploadID,
		Status:        dto.UploadStatus(session.Status),
		UploadedBytes: session.UploadedBytes,
	}

	s.logger.Info(
		"Queried upload status",
		zap.String("upload_id", uploadID),
		zap.Bool("refresh", req.Refresh),
		zap.String("trace_id", middleware.TraceIDFromContext(ctx)),
	)

	return response, nil
}

func (s *UploadService) CancelUploadSession(ctx context.Context, uploadID string, req dto.CancelUploadRequest) (dto.CancelUploadResponse, error) {
	if uploadID == "" {
		return dto.CancelUploadResponse{}, internalerrors.ErrInvalidInput
	}
	if s.store == nil {
		return dto.CancelUploadResponse{}, errors.New("missing upload session store")
	}

	session, err := s.store.GetByID(ctx, uploadID)
	if err != nil {
		return dto.CancelUploadResponse{}, fmt.Errorf("load upload session: %w", err)
	}
	if session == nil {
		return dto.CancelUploadResponse{}, internalerrors.ErrNotFound
	}

	if time.Now().UTC().After(session.ExpiresAt) {
		_ = s.store.MarkExpired(ctx, uploadID)
		return dto.CancelUploadResponse{}, internalerrors.ErrSessionExpired
	}

	if err := s.store.MarkCancelled(ctx, uploadID); err != nil {
		return dto.CancelUploadResponse{}, fmt.Errorf("cancel upload session: %w", err)
	}

	response := dto.CancelUploadResponse{
		UploadID: uploadID,
		Status:   dto.StatusCancelled,
	}

	s.logger.Info(
		"Cancelled upload session",
		zap.String("upload_id", uploadID),
		zap.String("reason", req.Reason),
		zap.String("trace_id", middleware.TraceIDFromContext(ctx)),
	)

	return response, nil
}

const defaultUploadTTL = 15 * time.Minute

func generateUploadID() string {
	value, err := uuid.NewV7()
	if err == nil {
		return "upl_" + value.String()
	}

	buffer := make([]byte, 16)
	if _, readErr := rand.Read(buffer); readErr != nil {
		return "upl_" + hex.EncodeToString([]byte(time.Now().UTC().Format("20060102150405")))
	}
	return "upl_" + hex.EncodeToString(buffer)
}

func buildObjectName(uploadID string, fileName string) string {
	if fileName == "" {
		return "uploads/" + uploadID
	}
	return "uploads/" + uploadID + "/" + fileName
}

func checksumAlgorithm(req *dto.ChecksumRequest) string {
	if req == nil {
		return ""
	}
	return req.Algorithm
}

func checksumValue(req *dto.ChecksumRequest) string {
	if req == nil {
		return ""
	}
	return req.Value
}

func mapSessionToCreateResponse(session *model.UploadSession) dto.CreateUploadResponse {
	return dto.CreateUploadResponse{
		UploadID:         session.UploadID,
		GCSUploadURL:     session.GCSUploadURL,
		Bucket:           session.Bucket,
		ObjectName:       session.ObjectName,
		SessionExpiresAt: session.ExpiresAt.Format(time.RFC3339),
		Status:           dto.UploadStatus(session.Status),
	}
}

func queryUploadedBytes(ctx context.Context, uploadURL string, sizeBytes int64) (int64, error) {
	if uploadURL == "" {
		return 0, errors.New("missing upload url")
	}
	if sizeBytes <= 0 {
		return 0, errors.New("invalid size bytes")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, uploadURL, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Range", fmt.Sprintf("bytes */%d", sizeBytes))
	req.Header.Set("Content-Length", "0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusPermanentRedirect || resp.StatusCode == 308 {
		rangeHeader := resp.Header.Get("Range")
		if rangeHeader == "" {
			return 0, nil
		}
		return parseUploadedBytes(rangeHeader)
	}
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusNoContent {
		return sizeBytes, nil
	}

	return 0, fmt.Errorf("unexpected status %d", resp.StatusCode)
}

func parseUploadedBytes(rangeHeader string) (int64, error) {
	trimmed := strings.TrimSpace(rangeHeader)
	if trimmed == "" {
		return 0, nil
	}
	trimmed = strings.TrimPrefix(trimmed, "bytes=")
	parts := strings.Split(trimmed, "-")
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid range header: %s", rangeHeader)
	}
	end, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid range end: %w", err)
	}
	return end + 1, nil
}

var _ UploadInterface = (*UploadService)(nil)
