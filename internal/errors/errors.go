// Package errors defines sentinel errors used across the service and handler
// layers for consistent error classification. Handlers map these to HTTP
// status codes; callers use errors.Is to distinguish error categories.
package errors

import "errors"

// ErrInvalidInput indicates that the caller provided invalid or missing input
// parameters. Handlers map this to HTTP 400 Bad Request.
var ErrInvalidInput = errors.New("invalid input")

// ErrNotFound indicates that the requested resource (e.g., upload session)
// does not exist. Handlers map this to HTTP 404 Not Found.
var ErrNotFound = errors.New("not found")

// ErrSessionExpired indicates that the upload session has exceeded its TTL
// and is no longer usable. Handlers map this to HTTP 410 Gone.
var ErrSessionExpired = errors.New("session expired")

// ErrCircuitOpen indicates that the circuit breaker is open and requests are
// being rejected to protect the downstream dependency. Handlers map this to
// HTTP 503 Service Unavailable.
var ErrCircuitOpen = errors.New("circuit breaker is open")
