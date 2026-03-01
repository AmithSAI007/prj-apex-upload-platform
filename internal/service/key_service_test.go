package service

import (
	"testing"

	"go.uber.org/zap"
)

// TestNewSMKeyService_Constructor verifies the constructor wires the logger
// and secrets client correctly.
func TestNewSMKeyService_Constructor(t *testing.T) {
	logger := zap.NewNop()
	svc := NewSMKeyService(logger, nil)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
	if svc.loggger != logger {
		t.Fatal("expected logger to be set")
	}
	if svc.client != nil {
		t.Fatal("expected nil client when constructed with nil")
	}
}

// TestKeyInterfaceCompile ensures SMKeyService satisfies KeyInterface at
// compile time.
func TestKeyInterfaceCompile(t *testing.T) {
	var _ KeyInterface = (*SMKeyService)(nil)
}
