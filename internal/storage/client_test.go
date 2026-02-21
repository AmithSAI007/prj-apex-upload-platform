package storage

import "testing"

func TestSignedURLClientInterface(t *testing.T) {
	var _ SignedURLClient = (*GCSClient)(nil)
}
