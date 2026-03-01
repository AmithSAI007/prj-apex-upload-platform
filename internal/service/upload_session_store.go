package service

import (
	"context"

	"github.com/AmithSAI007/prj-apex-upload-platform/internal/model"
)

// UploadSessionStore defines the persistence contract for upload session
// lifecycle operations. Implementations (e.g., FirestoreUploadSessionStore)
// handle the underlying database interactions.
type UploadSessionStore interface {
	// Create persists a new upload session. Returns an error if the document
	// already exists or the write fails.
	Create(ctx context.Context, session *model.UploadSession) error

	// GetByID retrieves a session by its unique upload ID. Returns (nil, ErrNotFound)
	// if the session does not exist.
	GetByID(ctx context.Context, uploadID string) (*model.UploadSession, error)

	// GetByIdempotencyKey looks up an existing session by the tenant+user+key
	// composite. Returns (nil, nil) if no matching session is found.
	GetByIdempotencyKey(ctx context.Context, tenantID string, userID string, idempotencyKey string) (*model.UploadSession, error)

	// UpdateStatus patches the session's status and uploaded byte count in the store.
	UpdateStatus(ctx context.Context, uploadID string, status model.UploadStatus, uploadedBytes int64) error

	// UpdateGCSUploadURL replaces the stored GCS upload URL (e.g., after a URL refresh).
	UpdateGCSUploadURL(ctx context.Context, uploadID string, gcsUploadURL string) error

	// MarkCompleted transitions the session to "completed" and records final byte count.
	MarkCompleted(ctx context.Context, uploadID string, uploadedBytes int64) error

	// MarkCancelled transitions the session to "cancelled".
	MarkCancelled(ctx context.Context, uploadID string) error

	// MarkExpired transitions the session to "expired" when its TTL has elapsed.
	MarkExpired(ctx context.Context, uploadID string) error
}
