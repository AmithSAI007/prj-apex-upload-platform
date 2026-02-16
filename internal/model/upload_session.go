package model

import "time"

// UploadStatus represents the lifecycle state of an upload session.
type UploadStatus string

const (
	StatusCreated    UploadStatus = "created"
	StatusInProgress UploadStatus = "in_progress"
	StatusCompleted  UploadStatus = "completed"
	StatusCancelled  UploadStatus = "cancelled"
	StatusFailed     UploadStatus = "failed"
	StatusExpired    UploadStatus = "expired"
)

// UploadSession is the Firestore persistence model for upload sessions.
type UploadSession struct {
	UploadID       string            `firestore:"uploadId"`
	TenantID       string            `firestore:"tenantId"`
	UserID         string            `firestore:"userId"`
	Bucket         string            `firestore:"bucket"`
	ObjectName     string            `firestore:"objectName"`
	ContentType    string            `firestore:"contentType"`
	SizeBytes      int64             `firestore:"sizeBytes"`
	Status         UploadStatus      `firestore:"status"`
	GCSUploadURL   string            `firestore:"gcsUploadUrl"`
	UploadedBytes  int64             `firestore:"uploadedBytes"`
	ChecksumAlg    string            `firestore:"checksumAlgorithm"`
	ChecksumValue  string            `firestore:"checksumValue"`
	Metadata       map[string]string `firestore:"metadata"`
	Labels         map[string]string `firestore:"labels"`
	IdempotencyKey string            `firestore:"idempotencyKey"`
	CreatedAt      time.Time         `firestore:"createdAt"`
	UpdatedAt      time.Time         `firestore:"updatedAt"`
	ExpiresAt      time.Time         `firestore:"expiresAt"`
}
