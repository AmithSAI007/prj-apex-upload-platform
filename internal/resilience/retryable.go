// Package resilience provides shared error classification helpers used by
// retry loops and circuit breakers throughout the service.
package resilience

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// retryableCodes is the set of gRPC status codes considered transient.
// These warrant automatic retry with backoff.
var retryableCodes = map[codes.Code]bool{
	codes.Unavailable:       true,
	codes.DeadlineExceeded:  true,
	codes.Aborted:           true,
	codes.ResourceExhausted: true,
	codes.Internal:          true,
}

// IsRetryable reports whether err is a transient gRPC error that should be
// retried. Non-gRPC errors (e.g., context cancellation) are not retryable.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	st, ok := status.FromError(err)
	if !ok {
		return false
	}
	return retryableCodes[st.Code()]
}

// IsRetryableHTTP reports whether the HTTP status code indicates a transient
// server error that should be retried. Retryable: 429 (Too Many Requests)
// and 5xx (server errors). All 4xx (except 429) are permanent.
func IsRetryableHTTP(statusCode int) bool {
	if statusCode == 429 {
		return true
	}
	return statusCode >= 500
}
