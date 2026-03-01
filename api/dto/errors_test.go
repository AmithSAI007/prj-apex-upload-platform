package dto

import "testing"

func TestErrorCodeValues(t *testing.T) {
	codes := []ErrorCode{
		ErrorCodeInvalidArgument,
		ErrorCodeUnauthorized,
		ErrorCodeForbidden,
		ErrorCodeNotFound,
		ErrorCodeGone,
		ErrorCodeConflict,
		ErrorCodeRateLimited,
		ErrorCodeNotImplemented,
		ErrorCodeInternal,
	}
	for _, code := range codes {
		if code == "" {
			t.Fatalf("expected error code to be non-empty")
		}
	}
}
