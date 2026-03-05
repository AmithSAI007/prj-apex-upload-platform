// Package dto defines the data transfer objects (request/response structs)
// used by the HTTP handlers. These structs are the API contract with clients
// and include JSON serialization tags and validation constraints.
package dto

// ChecksumRequest carries optional client-supplied checksum information for
// integrity verification. Supported algorithms are crc32c and md5.
type ChecksumRequest struct {
	Algorithm string `json:"algorithm" validate:"omitempty,oneof=crc32c md5" example:"crc32c"`
	Value     string `json:"value" validate:"omitempty" example:"8Qm5dA=="`
}

// CreateUploadRequest is the request body for POST /v1/uploads. It describes
// the file to be uploaded and optional metadata for the upload session.
type CreateUploadRequest struct {
	// FileName is the original name of the file being uploaded (required).
	FileName string `json:"fileName" validate:"required,safefilename" example:"invoice.pdf"`
	// ContentType is the MIME type of the file (required).
	ContentType string `json:"contentType" validate:"required,contenttype" example:"application/pdf"`
	// SizeBytes is the total file size in bytes; must be greater than zero.
	SizeBytes int64 `json:"sizeBytes" validate:"required,gt=0,maxfilesize" example:"10485760"`
	// Checksum is an optional client-supplied checksum for data integrity.
	Checksum *ChecksumRequest `json:"checksum" validate:"omitempty"`
	// Metadata is an optional set of arbitrary key-value pairs attached to the upload.
	Metadata map[string]string `json:"metadata" validate:"omitempty" example:"projectId:p123"`
	// Labels is an optional set of classification labels for the upload.
	Labels map[string]string `json:"labels" validate:"omitempty" example:"docType:invoice"`
	// IdempotencyKey is an optional client-supplied key used to deduplicate session creation.
	IdempotencyKey string `json:"idempotencyKey" validate:"omitempty" example:"4a5c2a9f-2d7e-4ef2-b7b4-2df51b7d9a0e"`
}

// ResumeUploadRequest is the request body for POST /v1/uploads/{uploadId}/resume.
type ResumeUploadRequest struct {
	// IdempotencyKey is an optional idempotency key for resume operations.
	IdempotencyKey string `json:"idempotencyKey" validate:"omitempty" example:"4a5c2a9f-2d7e-4ef2-b7b4-2df51b7d9a0e"`
}

// QueryStatusRequest is the request body for POST /v1/uploads/{uploadId}/status.
type QueryStatusRequest struct {
	// Refresh controls whether to query GCS for the latest uploaded byte count.
	// When false, only the Firestore-cached state is returned.
	Refresh bool `json:"refresh" example:"true"`
}

// CancelUploadRequest is the request body for POST /v1/uploads/{uploadId}/cancel.
type CancelUploadRequest struct {
	// Reason is an optional human-readable reason for the cancellation.
	Reason string `json:"reason" validate:"omitempty" example:"user_cancelled"`
}
