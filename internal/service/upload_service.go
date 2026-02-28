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

	"github.com/AmithSAI007/prj-apex-upload-platform/api/dto"
	"github.com/AmithSAI007/prj-apex-upload-platform/internal/config"
	internalerrors "github.com/AmithSAI007/prj-apex-upload-platform/internal/errors"
	"github.com/AmithSAI007/prj-apex-upload-platform/internal/model"
	"github.com/AmithSAI007/prj-apex-upload-platform/internal/storage"
	"github.com/AmithSAI007/prj-apex-upload-platform/pkg/constants"
	pkgtrace "github.com/AmithSAI007/prj-apex-upload-platform/pkg/trace"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
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
	tracer trace.Tracer
}

// NewUploadService constructs the upload service with dependencies.
func NewUploadService(logger *zap.Logger, gcsClient storage.SignedURLClient, cfg *config.Config, store UploadSessionStore, tracer trace.Tracer) *UploadService {
	return &UploadService{
		logger: logger,
		gcs:    gcsClient,
		cfg:    cfg,
		store:  store,
		tracer: tracer,
	}
}

// CreateUploadSession creates a new resumable upload session and persists it.
func (s *UploadService) CreateUploadSession(ctx context.Context, req dto.CreateUploadRequest) (dto.CreateUploadResponse, error) {
	startTime := time.Now().UTC()
	if req.FileName == "" || req.ContentType == "" || req.SizeBytes <= 0 {
		return dto.CreateUploadResponse{}, internalerrors.ErrInvalidInput
	}

	userID := pkgtrace.DataFromContext(ctx, string(constants.CtxUserIDKey))
	tenantID := pkgtrace.DataFromContext(ctx, string(constants.CtxTenantIDKey))

	ctx, span := s.tracer.Start(ctx, "CreateUploadSession")
	defer span.End()

	bucketName := s.cfg.GCSBucket
	if bucketName == "" {
		span.RecordError(errors.New("missing GCS bucket configuration"))
		span.SetStatus(codes.Error, "missing GCS signing configuration")
		return dto.CreateUploadResponse{}, errors.New("missing GCS signing configuration")
	}
	if s.store == nil {
		span.RecordError(errors.New("missing upload session store"))
		span.SetStatus(codes.Error, "missing upload session store")
		return dto.CreateUploadResponse{}, errors.New("missing upload session store")
	}

	if req.IdempotencyKey != "" {
		// use incoming request context as parent so spans link to the request trace
		rpcCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		idSpanCtx, idSpan := s.tracer.Start(rpcCtx, "LookupIdempotencyKey")
		defer idSpan.End()
		existing, err := s.store.GetByIdempotencyKey(idSpanCtx, tenantID, userID, req.IdempotencyKey)

		if err != nil {
			idSpan.RecordError(err)
			idSpan.SetStatus(codes.Error, "lookup idempotency key failed")
			span.RecordError(err)
			span.SetStatus(codes.Error, "lookup idempotency key failed")
			return dto.CreateUploadResponse{}, fmt.Errorf("lookup idempotency key: %w", err)
		}
		if existing != nil {
			s.logger.Info("Reusing upload session for idempotency key", zap.String("upload_id", existing.UploadID))
			idSpan.SetAttributes(attribute.String("idempotency_key", req.IdempotencyKey), attribute.String("upload_id", existing.UploadID))
			idSpan.SetStatus(codes.Ok, "found existing upload session for idempotency key")
			span.SetAttributes(attribute.String("idempotency_key", req.IdempotencyKey), attribute.String("upload_id", existing.UploadID))
			span.SetStatus(codes.Ok, "reused existing upload session for idempotency key")
			return mapSessionToCreateResponse(existing), nil
		}
	}

	uploadID := generateUploadID()
	objectName := buildObjectName(uploadID, req.FileName)
	signCtx, signSpan := s.tracer.Start(ctx, "SignResumableUploadURL")
	defer signSpan.End()
	signSpan.SetAttributes(attribute.String("bucket", bucketName), attribute.String("object_name", objectName))
	signedURL, err := s.gcs.SignResumableUploadURL(signCtx, bucketName, objectName, s.cfg.SERVICE_ACCOUNT_EMAIL)
	if err != nil {
		signSpan.RecordError(err)
		signSpan.SetStatus(codes.Error, "create signed resumable upload url failed")
		span.SetAttributes(attribute.String("error", err.Error()))
		span.SetStatus(codes.Error, "create signed resumable upload url failed")
		return dto.CreateUploadResponse{}, fmt.Errorf("create signed resumable upload url: %w", err)
	}

	createdAt := startTime
	expiresAt := startTime.Add(defaultUploadTTL)
	session := &model.UploadSession{
		UploadID:       uploadID,
		TenantID:       tenantID,
		UserID:         userID,
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

	persistCtx, persistSpan := s.tracer.Start(ctx, "PersistUploadSession")
	persistSpan.SetAttributes(attribute.String("upload_id", uploadID), attribute.Int64("size_bytes", req.SizeBytes))
	if err := s.store.Create(persistCtx, session); err != nil {
		persistSpan.SetAttributes(attribute.String("error", err.Error()))
		persistSpan.SetStatus(codes.Error, "persist upload session failed")
		persistSpan.End()
		span.SetAttributes(attribute.String("error", err.Error()))
		span.SetStatus(codes.Error, "persist upload session failed")
		return dto.CreateUploadResponse{}, fmt.Errorf("persist upload session: %w", err)
	}
	persistSpan.End()

	response := mapSessionToCreateResponse(session)

	span.SetAttributes(
		attribute.String("upload_id", uploadID),
		attribute.String("object_name", objectName),
		attribute.String("bucket", bucketName),
		attribute.Int64("size_bytes", req.SizeBytes),
		attribute.String("user_id", userID),
		attribute.String("trace_id", pkgtrace.TraceIDFromContext(ctx)),
	)

	s.logger.Info(
		"Created upload session",
		zap.String("upload_id", uploadID),
		zap.String("object_name", objectName),
		zap.String("bucket", bucketName),
		zap.Int64("size_bytes", req.SizeBytes),
		zap.String("trace_id", pkgtrace.TraceIDFromContext(ctx)),
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

	ctx, span := s.tracer.Start(ctx, "ResumeUploadSession")
	defer span.End()

	span.SetAttributes(
		attribute.String("upload_id", uploadID),
		attribute.String("trace_id", pkgtrace.TraceIDFromContext(ctx)),
		attribute.String("user_id", pkgtrace.DataFromContext(ctx, string(constants.CtxUserIDKey))),
		attribute.String("tenant_id", pkgtrace.DataFromContext(ctx, string(constants.CtxTenantIDKey))),
	)

	// load session
	loadCtx, loadSpan := s.tracer.Start(ctx, "LoadUploadSession")
	session, err := s.store.GetByID(loadCtx, uploadID)
	if err != nil {
		loadSpan.RecordError(err)
		loadSpan.SetAttributes(attribute.String("error", err.Error()))
		loadSpan.SetStatus(codes.Error, "load upload session failed")
		loadSpan.End()
		span.SetAttributes(attribute.String("error", err.Error()))
		span.SetStatus(codes.Error, "load upload session failed")
		return dto.ResumeUploadResponse{}, fmt.Errorf("load upload session: %w", err)
	}
	loadSpan.End()

	if session == nil {
		span.RecordError(errors.New("session not found"))
		span.SetAttributes(attribute.String("error", "session not found"))
		span.SetStatus(codes.Error, "session not found")
		return dto.ResumeUploadResponse{}, internalerrors.ErrNotFound
	}

	if time.Now().UTC().After(session.ExpiresAt) {
		// mark expired
		expCtx, expSpan := s.tracer.Start(ctx, "MarkExpired")
		_ = s.store.MarkExpired(expCtx, uploadID)
		expSpan.End()
		span.SetAttributes(attribute.String("error", "session expired"))
		span.SetStatus(codes.Error, "session expired")
		return dto.ResumeUploadResponse{}, internalerrors.ErrSessionExpired
	}

	response := dto.ResumeUploadResponse{
		UploadID:         session.UploadID,
		GCSUploadURL:     session.GCSUploadURL,
		SessionExpiresAt: session.ExpiresAt.Format(time.RFC3339),
		Status:           dto.UploadStatus(session.Status),
	}

	span.SetStatus(codes.Ok, "resumed upload session")
	s.logger.Info("Resumed upload session", zap.String("upload_id", uploadID), zap.String("trace_id", pkgtrace.TraceIDFromContext(ctx)))
	return response, nil
}

func (s *UploadService) GetUploadStatus(ctx context.Context, uploadID string) (dto.UploadStatusResponse, error) {
	if uploadID == "" {
		return dto.UploadStatusResponse{}, internalerrors.ErrInvalidInput
	}
	if s.store == nil {
		return dto.UploadStatusResponse{}, errors.New("missing upload session store")
	}

	ctx, span := s.tracer.Start(ctx, "GetUploadStatus")
	defer span.End()

	span.SetAttributes(
		attribute.String("upload_id", uploadID),
		attribute.String("trace_id", pkgtrace.TraceIDFromContext(ctx)),
		attribute.String("user_id", pkgtrace.DataFromContext(ctx, string(constants.CtxUserIDKey))),
		attribute.String("tenant_id", pkgtrace.DataFromContext(ctx, string(constants.CtxTenantIDKey))),
	)

	loadCtx, loadSpan := s.tracer.Start(ctx, "LoadUploadSession")
	session, err := s.store.GetByID(loadCtx, uploadID)
	if err != nil {
		loadSpan.RecordError(err)
		loadSpan.SetAttributes(attribute.String("error", err.Error()))
		loadSpan.SetStatus(codes.Error, "load upload session failed")
		loadSpan.End()
		span.SetAttributes(attribute.String("error", err.Error()))
		span.SetStatus(codes.Error, "load upload session failed")
		return dto.UploadStatusResponse{}, fmt.Errorf("load upload session: %w", err)
	}
	loadSpan.End()

	if session == nil {
		span.RecordError(errors.New("session not found"))
		span.SetAttributes(attribute.String("error", "session not found"))
		span.SetStatus(codes.Error, "session not found")
		return dto.UploadStatusResponse{}, internalerrors.ErrNotFound
	}

	if time.Now().UTC().After(session.ExpiresAt) {
		expCtx, expSpan := s.tracer.Start(ctx, "MarkExpired")
		_ = s.store.MarkExpired(expCtx, uploadID)
		expSpan.End()
		span.SetAttributes(attribute.String("error", "session expired"))
		span.SetStatus(codes.Error, "session expired")
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

	span.SetStatus(codes.Ok, "fetched upload status")
	s.logger.Info("Fetched upload status", zap.String("upload_id", uploadID), zap.String("trace_id", pkgtrace.TraceIDFromContext(ctx)))
	return response, nil
}

func (s *UploadService) QueryUploadStatus(ctx context.Context, uploadID string, req dto.QueryStatusRequest) (dto.QueryStatusResponse, error) {
	if uploadID == "" {
		return dto.QueryStatusResponse{}, internalerrors.ErrInvalidInput
	}
	if s.store == nil {
		return dto.QueryStatusResponse{}, errors.New("missing upload session store")
	}

	ctx, span := s.tracer.Start(ctx, "QueryUploadStatus")
	defer span.End()

	span.SetAttributes(
		attribute.String("upload_id", uploadID),
		attribute.Bool("refresh", req.Refresh),
		attribute.String("trace_id", pkgtrace.TraceIDFromContext(ctx)),
		attribute.String("user_id", pkgtrace.DataFromContext(ctx, string(constants.CtxUserIDKey))),
		attribute.String("tenant_id", pkgtrace.DataFromContext(ctx, string(constants.CtxTenantIDKey))),
	)

	loadCtx, loadSpan := s.tracer.Start(ctx, "LoadUploadSession")
	session, err := s.store.GetByID(loadCtx, uploadID)
	if err != nil {
		loadSpan.RecordError(err)
		loadSpan.SetAttributes(attribute.String("error", err.Error()))
		loadSpan.SetStatus(codes.Error, "load upload session failed")
		loadSpan.End()
		span.SetAttributes(attribute.String("error", err.Error()))
		span.SetStatus(codes.Error, "load upload session failed")
		return dto.QueryStatusResponse{}, fmt.Errorf("load upload session: %w", err)
	}
	loadSpan.End()

	if session == nil {
		span.RecordError(errors.New("session not found"))
		span.SetAttributes(attribute.String("error", "session not found"))
		span.SetStatus(codes.Error, "session not found")
		return dto.QueryStatusResponse{}, internalerrors.ErrNotFound
	}

	if time.Now().UTC().After(session.ExpiresAt) {
		expCtx, expSpan := s.tracer.Start(ctx, "MarkExpired")
		_ = s.store.MarkExpired(expCtx, uploadID)
		expSpan.End()
		span.SetAttributes(attribute.String("error", "session expired"))
		span.SetStatus(codes.Error, "session expired")
		return dto.QueryStatusResponse{}, internalerrors.ErrSessionExpired
	}

	if req.Refresh {
		// only query GCS when we have a signed upload url
		if session.GCSUploadURL == "" {
			// nothing to refresh from GCS, return current stored bytes
		} else {
			// query GCS
			qCtx, qSpan := s.tracer.Start(ctx, "QueryGCSUploadedBytes")
			uploadedBytes, err := queryUploadedBytes(qCtx, session.GCSUploadURL, session.SizeBytes)
			if err != nil {
				qSpan.RecordError(err)
				qSpan.SetAttributes(attribute.String("error", err.Error()))
				qSpan.SetStatus(codes.Error, "query gcs upload status failed")
				qSpan.End()
				span.RecordError(err)
				span.SetAttributes(attribute.String("error", err.Error()))
				span.SetStatus(codes.Error, "query gcs upload status failed")
				return dto.QueryStatusResponse{}, fmt.Errorf("query gcs upload status: %w", err)
			}
			qSpan.End()

			if uploadedBytes >= session.SizeBytes {
				mCtx, mSpan := s.tracer.Start(ctx, "MarkCompleted")
				if err := s.store.MarkCompleted(mCtx, uploadID, session.SizeBytes); err != nil {
					mSpan.RecordError(err)
					mSpan.SetAttributes(attribute.String("error", err.Error()))
					mSpan.SetStatus(codes.Error, "mark upload completed failed")
					mSpan.End()
					span.RecordError(err)
					span.SetAttributes(attribute.String("error", err.Error()))
					span.SetStatus(codes.Error, "mark upload completed failed")
					return dto.QueryStatusResponse{}, fmt.Errorf("mark upload completed: %w", err)
				}
				mSpan.End()
				session.Status = model.StatusCompleted
				session.UploadedBytes = session.SizeBytes
			} else {
				uCtx, uSpan := s.tracer.Start(ctx, "UpdateStatus")
				if err := s.store.UpdateStatus(uCtx, uploadID, session.Status, uploadedBytes); err != nil {
					uSpan.RecordError(err)
					uSpan.SetAttributes(attribute.String("error", err.Error()))
					uSpan.SetStatus(codes.Error, "update upload status failed")
					uSpan.End()
					span.RecordError(err)
					span.SetAttributes(attribute.String("error", err.Error()))
					span.SetStatus(codes.Error, "update upload status failed")
					return dto.QueryStatusResponse{}, fmt.Errorf("update upload status: %w", err)
				}
				uSpan.End()
				session.UploadedBytes = uploadedBytes
			}
		}
	}

	response := dto.QueryStatusResponse{
		UploadID:      session.UploadID,
		Status:        dto.UploadStatus(session.Status),
		UploadedBytes: session.UploadedBytes,
	}

	span.SetStatus(codes.Ok, "queried upload status")
	s.logger.Info("Queried upload status", zap.String("upload_id", uploadID), zap.Bool("refresh", req.Refresh), zap.String("trace_id", pkgtrace.TraceIDFromContext(ctx)))
	return response, nil
}

func (s *UploadService) CancelUploadSession(ctx context.Context, uploadID string, req dto.CancelUploadRequest) (dto.CancelUploadResponse, error) {
	if uploadID == "" {
		return dto.CancelUploadResponse{}, internalerrors.ErrInvalidInput
	}
	if s.store == nil {
		return dto.CancelUploadResponse{}, errors.New("missing upload session store")
	}

	ctx, span := s.tracer.Start(ctx, "CancelUploadSession")
	defer span.End()

	span.SetAttributes(
		attribute.String("upload_id", uploadID),
		attribute.String("trace_id", pkgtrace.TraceIDFromContext(ctx)),
		attribute.String("user_id", pkgtrace.DataFromContext(ctx, string(constants.CtxUserIDKey))),
		attribute.String("tenant_id", pkgtrace.DataFromContext(ctx, string(constants.CtxTenantIDKey))),
	)

	loadCtx, loadSpan := s.tracer.Start(ctx, "LoadUploadSession")
	session, err := s.store.GetByID(loadCtx, uploadID)
	if err != nil {
		loadSpan.SetAttributes(attribute.String("error", err.Error()))
		loadSpan.SetStatus(codes.Error, "load upload session failed")
		loadSpan.End()
		span.SetAttributes(attribute.String("error", err.Error()))
		span.SetStatus(codes.Error, "load upload session failed")
		return dto.CancelUploadResponse{}, fmt.Errorf("load upload session: %w", err)
	}
	loadSpan.End()

	if session == nil {
		span.SetAttributes(attribute.String("error", "session not found"))
		span.SetStatus(codes.Error, "session not found")
		return dto.CancelUploadResponse{}, internalerrors.ErrNotFound
	}

	if time.Now().UTC().After(session.ExpiresAt) {
		expCtx, expSpan := s.tracer.Start(ctx, "MarkExpired")
		_ = s.store.MarkExpired(expCtx, uploadID)
		expSpan.End()
		span.SetAttributes(attribute.String("error", "session expired"))
		span.SetStatus(codes.Error, "session expired")
		return dto.CancelUploadResponse{}, internalerrors.ErrSessionExpired
	}

	cancelCtx, cancelSpan := s.tracer.Start(ctx, "MarkCancelled")
	if err := s.store.MarkCancelled(cancelCtx, uploadID); err != nil {
		cancelSpan.SetAttributes(attribute.String("error", err.Error()))
		cancelSpan.SetStatus(codes.Error, "cancel upload session failed")
		cancelSpan.End()
		span.SetAttributes(attribute.String("error", err.Error()))
		span.SetStatus(codes.Error, "cancel upload session failed")
		return dto.CancelUploadResponse{}, fmt.Errorf("cancel upload session: %w", err)
	}
	cancelSpan.End()

	response := dto.CancelUploadResponse{
		UploadID: uploadID,
		Status:   dto.StatusCancelled,
	}

	span.SetStatus(codes.Ok, "cancelled upload session")
	s.logger.Info("Cancelled upload session", zap.String("upload_id", uploadID), zap.String("reason", req.Reason), zap.String("trace_id", pkgtrace.TraceIDFromContext(ctx)))
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
