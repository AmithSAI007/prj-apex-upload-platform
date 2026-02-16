package service

import (
	"context"

	"github.com/AmithSAI007/prj-apex-upload-platform/internal/model"
)

// UploadSessionStore defines persistence operations for upload sessions.
type UploadSessionStore interface {
	Create(ctx context.Context, session *model.UploadSession) error
	GetByID(ctx context.Context, uploadID string) (*model.UploadSession, error)
	GetByIdempotencyKey(ctx context.Context, tenantID string, userID string, idempotencyKey string) (*model.UploadSession, error)
	UpdateStatus(ctx context.Context, uploadID string, status model.UploadStatus, uploadedBytes int64) error
	UpdateGCSUploadURL(ctx context.Context, uploadID string, gcsUploadURL string) error
	MarkCompleted(ctx context.Context, uploadID string, uploadedBytes int64) error
	MarkCancelled(ctx context.Context, uploadID string) error
	MarkExpired(ctx context.Context, uploadID string) error
}
