package storage

import (
	"context"
	"errors"
	"fmt"

	internalerrors "github.com/AmithSAI007/prj-apex-upload-platform/internal/errors"
	"github.com/sony/gobreaker/v2"
)

// CBSignedURLClient is a circuit breaker decorator around a SignedURLClient
// implementation. It delegates every call through a gobreaker CircuitBreaker,
// rejecting requests with ErrCircuitOpen when the breaker is open. The call
// chain is:
//
//	Service → CBSignedURLClient → inner (GCSClient with retry)
type CBSignedURLClient struct {
	inner SignedURLClient
	cb    *gobreaker.CircuitBreaker[string]
}

// NewCBSignedURLClient wraps inner with a circuit breaker using the given
// gobreaker settings. The caller is responsible for configuring ReadyToTrip,
// OnStateChange, and other breaker policies via the settings.
func NewCBSignedURLClient(inner SignedURLClient, settings gobreaker.Settings) *CBSignedURLClient {
	return &CBSignedURLClient{
		inner: inner,
		cb:    gobreaker.NewCircuitBreaker[string](settings),
	}
}

func (c *CBSignedURLClient) SignResumableUploadURL(ctx context.Context, bucket string, objectName string, serviceAccount string) (string, error) {
	result, err := c.cb.Execute(func() (string, error) {
		return c.inner.SignResumableUploadURL(ctx, bucket, objectName, serviceAccount)
	})
	if err != nil {
		if errors.Is(err, gobreaker.ErrOpenState) || errors.Is(err, gobreaker.ErrTooManyRequests) {
			return "", fmt.Errorf("%w: %w", internalerrors.ErrCircuitOpen, err)
		}
		return "", err
	}
	return result, nil
}

var _ SignedURLClient = (*CBSignedURLClient)(nil)
