package dto

// ErrorCode represents standardized error codes.
type ErrorCode string

const (
	ErrorCodeInvalidArgument ErrorCode = "invalid_argument"
	ErrorCodeUnauthorized    ErrorCode = "unauthorized"
	ErrorCodeForbidden       ErrorCode = "forbidden"
	ErrorCodeNotFound        ErrorCode = "not_found"
	ErrorCodeConflict        ErrorCode = "conflict"
	ErrorCodeRateLimited     ErrorCode = "rate_limited"
	ErrorCodeNotImplemented  ErrorCode = "not_implemented"
	ErrorCodeInternal        ErrorCode = "internal"
)

type ErrorDetail struct {
	Field   string `json:"field,omitempty" example:"fileName"`
	Message string `json:"message" example:"fileName is required"`
}

type ErrorResponse struct {
	Error ErrorPayload `json:"error"`
}

type ErrorPayload struct {
	Code      ErrorCode     `json:"code" example:"invalid_argument"`
	Message   string        `json:"message" example:"Validation failed"`
	RequestID string        `json:"requestId,omitempty" example:"req_6f1a2c9d5e7b3a1c"`
	Details   []ErrorDetail `json:"details,omitempty"`
}
