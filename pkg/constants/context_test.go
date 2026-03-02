package constants

import "testing"

func TestContextKeyValues(t *testing.T) {
	tests := []struct {
		name     string
		key      CtxKey
		expected string
	}{
		{name: "CtxUserIDKey", key: CtxUserIDKey, expected: "user_id"},
		{name: "CtxTenantIDKey", key: CtxTenantIDKey, expected: "tenant_id"},
		{name: "CtxTraceIDKey", key: CtxTraceIDKey, expected: "trace_id"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.key) != tt.expected {
				t.Fatalf("expected %s, got %s", tt.expected, string(tt.key))
			}
		})
	}
}

func TestContextKeyType_PreventsCollision(t *testing.T) {
	// Ensure CtxKey("user_id") != plain string "user_id" at the type level.
	// This is enforced by the Go type system, but we verify the key is the
	// correct typed value.
	var key interface{} = CtxUserIDKey
	if _, ok := key.(string); ok {
		t.Fatal("CtxKey should not be assignable to plain string")
	}
}
