package dto

// UploadStatus represents the lifecycle state of an upload session as returned
// in API responses. Mirrors the internal model.UploadStatus values.
type UploadStatus string

const (
	// StatusCreated indicates the session was just created and no bytes have been uploaded.
	StatusCreated UploadStatus = "created"
	// StatusInProgress indicates the client is actively uploading data.
	StatusInProgress UploadStatus = "in_progress"
	// StatusCompleted indicates all bytes have been uploaded and verified.
	StatusCompleted UploadStatus = "completed"
	// StatusCancelled indicates the upload was explicitly cancelled by the client.
	StatusCancelled UploadStatus = "cancelled"
	// StatusFailed indicates the upload failed due to an unrecoverable error.
	StatusFailed UploadStatus = "failed"
	// StatusExpired indicates the session exceeded its TTL before completion.
	StatusExpired UploadStatus = "expired"
)

// CreateUploadResponse is returned by POST /v1/uploads on success (201).
type CreateUploadResponse struct {
	UploadID         string       `json:"uploadId" example:"upl_0192f2c1-6a7c-79b2-8f73-2d3c3a4b5c6d"`
	GCSUploadURL     string       `json:"gcsUploadUrl" example:"https://storage.googleapis.com/upload/storage/v1/b/my-bucket/o?uploadType=resumable&upload_id=..."`
	Bucket           string       `json:"bucket" example:"my-bucket"`
	ObjectName       string       `json:"objectName" example:"uploads/upl_6f1a2c9d5e7b3a1c/invoice.pdf"`
	SessionExpiresAt string       `json:"sessionExpiresAt" example:"2026-02-15T17:15:00Z"`
	Status           UploadStatus `json:"status" example:"created"`
}

// ResumeUploadResponse is returned by POST /v1/uploads/{uploadId}/resume.
type ResumeUploadResponse struct {
	UploadID         string       `json:"uploadId" example:"upl_0192f2c1-6a7c-79b2-8f73-2d3c3a4b5c6d"`
	GCSUploadURL     string       `json:"gcsUploadUrl" example:"https://storage.googleapis.com/upload/storage/v1/b/my-bucket/o?uploadType=resumable&upload_id=..."`
	SessionExpiresAt string       `json:"sessionExpiresAt" example:"2026-02-15T17:15:00Z"`
	Status           UploadStatus `json:"status" example:"in_progress"`
}

// UploadStatusResponse is returned by GET /v1/uploads/{uploadId}.
type UploadStatusResponse struct {
	UploadID      string       `json:"uploadId" example:"upl_0192f2c1-6a7c-79b2-8f73-2d3c3a4b5c6d"`
	Status        UploadStatus `json:"status" example:"in_progress"`
	SizeBytes     int64        `json:"sizeBytes" example:"10485760"`
	ContentType   string       `json:"contentType" example:"application/pdf"`
	ObjectName    string       `json:"objectName" example:"uploads/upl_6f1a2c9d5e7b3a1c/invoice.pdf"`
	CreatedAt     string       `json:"createdAt" example:"2026-02-15T17:10:00Z"`
	UpdatedAt     string       `json:"updatedAt" example:"2026-02-15T17:12:00Z"`
	UploadedBytes int64        `json:"uploadedBytes" example:"5242880"`
}

// QueryStatusResponse is returned by POST /v1/uploads/{uploadId}/status.
type QueryStatusResponse struct {
	UploadID      string       `json:"uploadId" example:"upl_0192f2c1-6a7c-79b2-8f73-2d3c3a4b5c6d"`
	Status        UploadStatus `json:"status" example:"in_progress"`
	UploadedBytes int64        `json:"uploadedBytes" example:"5242880"`
}

// CancelUploadResponse is returned by POST /v1/uploads/{uploadId}/cancel.
type CancelUploadResponse struct {
	UploadID string       `json:"uploadId" example:"upl_0192f2c1-6a7c-79b2-8f73-2d3c3a4b5c6d"`
	Status   UploadStatus `json:"status" example:"cancelled"`
}
