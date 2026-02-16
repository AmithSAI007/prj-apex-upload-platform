package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
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
	gcs    *storage.GCSClient
	cfg    *config.Config
	store  UploadSessionStore
}

// NewUploadService constructs the upload service with dependencies.
func NewUploadService(logger *zap.Logger, gcsClient *storage.GCSClient, cfg *config.Config, store UploadSessionStore) *UploadService {
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
	return dto.ResumeUploadResponse{}, nil
}

func (s *UploadService) GetUploadStatus(ctx context.Context, uploadID string) (dto.UploadStatusResponse, error) {
	return dto.UploadStatusResponse{}, nil
}

func (s *UploadService) QueryUploadStatus(ctx context.Context, uploadID string, req dto.QueryStatusRequest) (dto.QueryStatusResponse, error) {
	return dto.QueryStatusResponse{}, nil
}

func (s *UploadService) CancelUploadSession(ctx context.Context, uploadID string, req dto.CancelUploadRequest) (dto.CancelUploadResponse, error) {
	return dto.CancelUploadResponse{}, nil
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

var _ UploadInterface = (*UploadService)(nil)
