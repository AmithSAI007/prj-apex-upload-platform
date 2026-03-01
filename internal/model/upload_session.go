// Package model defines the Firestore persistence models used by the service
// layer. These structs map directly to Firestore documents via struct tags.
package model

import "time"

// UploadStatus represents the lifecycle state of an upload session stored
// in Firestore. Transitions follow: created -> in_progress -> completed/cancelled/failed/expired.
type UploadStatus string

const (
	// StatusCreated means the session was created but no bytes have been uploaded yet.
	StatusCreated UploadStatus = "created"
	// StatusInProgress means the client is actively uploading data to GCS.
	StatusInProgress UploadStatus = "in_progress"
	// StatusCompleted means all bytes have been uploaded and the upload is finalized.
	StatusCompleted UploadStatus = "completed"
	// StatusCancelled means the upload was explicitly cancelled by the client.
	StatusCancelled UploadStatus = "cancelled"
	// StatusFailed means the upload failed due to an unrecoverable error.
	StatusFailed UploadStatus = "failed"
	// StatusExpired means the session TTL elapsed before the upload completed.
	StatusExpired UploadStatus = "expired"
)

// UploadSession is the Firestore document model for an upload session.
// Each session maps to a single GCS resumable upload and is stored in the
// "upload_sessions" collection keyed by UploadID.
type UploadSession struct {
	// UploadID is the unique identifier for this upload session (UUIDv7-based, prefixed with "upl_").
	UploadID string `firestore:"uploadId"`
	// TenantID identifies the tenant that owns this upload for multi-tenant isolation.
	TenantID string `firestore:"tenantId"`
	// UserID identifies the authenticated user who initiated the upload.
	UserID string `firestore:"userId"`
	// Bucket is the GCS bucket where the object is being uploaded.
	Bucket string `firestore:"bucket"`
	// ObjectName is the full GCS object path (e.g., "uploads/<uploadID>/<fileName>").
	ObjectName string `firestore:"objectName"`
	// ContentType is the MIME type of the file being uploaded.
	ContentType string `firestore:"contentType"`
	// SizeBytes is the total expected file size in bytes.
	SizeBytes int64 `firestore:"sizeBytes"`
	// Status is the current lifecycle state of the upload session.
	Status UploadStatus `firestore:"status"`
	// GCSUploadURL is the signed resumable upload URL issued by GCS.
	GCSUploadURL string `firestore:"gcsUploadUrl"`
	// UploadedBytes tracks how many bytes have been uploaded so far.
	UploadedBytes int64 `firestore:"uploadedBytes"`
	// ChecksumAlg is the checksum algorithm (e.g., "crc32c", "md5") if client-supplied.
	ChecksumAlg string `firestore:"checksumAlgorithm"`
	// ChecksumValue is the expected checksum value for data integrity verification.
	ChecksumValue string `firestore:"checksumValue"`
	// Metadata holds arbitrary key-value pairs attached to the upload by the client.
	Metadata map[string]string `firestore:"metadata"`
	// Labels holds classification labels for the upload.
	Labels map[string]string `firestore:"labels"`
	// IdempotencyKey is a client-supplied key used to deduplicate session creation.
	IdempotencyKey string `firestore:"idempotencyKey"`
	// CreatedAt is the timestamp when the session was created.
	CreatedAt time.Time `firestore:"createdAt"`
	// UpdatedAt is the timestamp of the last update to the session.
	UpdatedAt time.Time `firestore:"updatedAt"`
	// ExpiresAt is the timestamp after which the session is considered expired.
	ExpiresAt time.Time `firestore:"expiresAt"`
}
