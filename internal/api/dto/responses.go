package dto

type UploadStatus string

const (
	StatusCreated    UploadStatus = "created"
	StatusInProgress UploadStatus = "in_progress"
	StatusCompleted  UploadStatus = "completed"
	StatusCancelled  UploadStatus = "cancelled"
	StatusFailed     UploadStatus = "failed"
	StatusExpired    UploadStatus = "expired"
)

type CreateUploadResponse struct {
	UploadID         string       `json:"uploadId" example:"upl_0192f2c1-6a7c-79b2-8f73-2d3c3a4b5c6d"`
	GCSUploadURL     string       `json:"gcsUploadUrl" example:"https://storage.googleapis.com/upload/storage/v1/b/my-bucket/o?uploadType=resumable&upload_id=..."`
	Bucket           string       `json:"bucket" example:"my-bucket"`
	ObjectName       string       `json:"objectName" example:"uploads/upl_6f1a2c9d5e7b3a1c/invoice.pdf"`
	SessionExpiresAt string       `json:"sessionExpiresAt" example:"2026-02-15T17:15:00Z"`
	Status           UploadStatus `json:"status" example:"created"`
}

type ResumeUploadResponse struct {
	UploadID         string       `json:"uploadId" example:"upl_0192f2c1-6a7c-79b2-8f73-2d3c3a4b5c6d"`
	GCSUploadURL     string       `json:"gcsUploadUrl" example:"https://storage.googleapis.com/upload/storage/v1/b/my-bucket/o?uploadType=resumable&upload_id=..."`
	SessionExpiresAt string       `json:"sessionExpiresAt" example:"2026-02-15T17:15:00Z"`
	Status           UploadStatus `json:"status" example:"in_progress"`
}

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

type QueryStatusResponse struct {
	UploadID      string       `json:"uploadId" example:"upl_0192f2c1-6a7c-79b2-8f73-2d3c3a4b5c6d"`
	Status        UploadStatus `json:"status" example:"in_progress"`
	UploadedBytes int64        `json:"uploadedBytes" example:"5242880"`
}

type CancelUploadResponse struct {
	UploadID string       `json:"uploadId" example:"upl_0192f2c1-6a7c-79b2-8f73-2d3c3a4b5c6d"`
	Status   UploadStatus `json:"status" example:"cancelled"`
}
