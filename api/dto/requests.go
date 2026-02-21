package dto

type ChecksumRequest struct {
	Algorithm string `json:"algorithm" validate:"omitempty,oneof=crc32c md5" example:"crc32c"`
	Value     string `json:"value" validate:"omitempty" example:"8Qm5dA=="`
}

type CreateUploadRequest struct {
	FileName       string            `json:"fileName" validate:"required" example:"invoice.pdf"`
	ContentType    string            `json:"contentType" validate:"required" example:"application/pdf"`
	SizeBytes      int64             `json:"sizeBytes" validate:"required,gt=0" example:"10485760"`
	Checksum       *ChecksumRequest  `json:"checksum" validate:"omitempty"`
	Metadata       map[string]string `json:"metadata" validate:"omitempty" example:"projectId:p123"`
	Labels         map[string]string `json:"labels" validate:"omitempty" example:"docType:invoice"`
	IdempotencyKey string            `json:"idempotencyKey" validate:"omitempty" example:"4a5c2a9f-2d7e-4ef2-b7b4-2df51b7d9a0e"`
}

type ResumeUploadRequest struct {
	IdempotencyKey string `json:"idempotencyKey" validate:"omitempty" example:"4a5c2a9f-2d7e-4ef2-b7b4-2df51b7d9a0e"`
}

type QueryStatusRequest struct {
	Refresh bool `json:"refresh" example:"true"`
}

type CancelUploadRequest struct {
	Reason string `json:"reason" validate:"omitempty" example:"user_cancelled"`
}
