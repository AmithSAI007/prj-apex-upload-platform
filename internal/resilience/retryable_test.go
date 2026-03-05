package resilience

import (
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestIsRetryable_NilError(t *testing.T) {
	if IsRetryable(nil) {
		t.Fatal("nil error should not be retryable")
	}
}

func TestIsRetryable_NonGRPCError(t *testing.T) {
	if IsRetryable(errors.New("plain error")) {
		t.Fatal("non-gRPC error should not be retryable")
	}
}

func TestIsRetryable_RetryableCodes(t *testing.T) {
	retryable := []codes.Code{
		codes.Unavailable,
		codes.DeadlineExceeded,
		codes.Aborted,
		codes.ResourceExhausted,
		codes.Internal,
	}
	for _, code := range retryable {
		err := status.Error(code, "test")
		if !IsRetryable(err) {
			t.Fatalf("expected code %v to be retryable", code)
		}
	}
}

func TestIsRetryable_PermanentCodes(t *testing.T) {
	permanent := []codes.Code{
		codes.NotFound,
		codes.AlreadyExists,
		codes.PermissionDenied,
		codes.InvalidArgument,
		codes.Unauthenticated,
		codes.FailedPrecondition,
		codes.Unimplemented,
		codes.OutOfRange,
		codes.DataLoss,
		codes.OK,
	}
	for _, code := range permanent {
		err := status.Error(code, "test")
		if IsRetryable(err) {
			t.Fatalf("expected code %v to be permanent (not retryable)", code)
		}
	}
}

func TestIsRetryableHTTP_429(t *testing.T) {
	if !IsRetryableHTTP(429) {
		t.Fatal("429 should be retryable")
	}
}

func TestIsRetryableHTTP_5xx(t *testing.T) {
	for _, code := range []int{500, 502, 503, 504} {
		if !IsRetryableHTTP(code) {
			t.Fatalf("expected %d to be retryable", code)
		}
	}
}

func TestIsRetryableHTTP_4xx(t *testing.T) {
	for _, code := range []int{400, 401, 403, 404, 409} {
		if IsRetryableHTTP(code) {
			t.Fatalf("expected %d to be permanent (not retryable)", code)
		}
	}
}

func TestIsRetryableHTTP_2xx(t *testing.T) {
	for _, code := range []int{200, 201, 204} {
		if IsRetryableHTTP(code) {
			t.Fatalf("expected %d to be non-retryable (success)", code)
		}
	}
}
