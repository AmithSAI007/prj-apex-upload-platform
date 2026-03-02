package errors

import (
	"errors"
	"fmt"
	"testing"
)

func TestSentinelErrors_NotNil(t *testing.T) {
	sentinels := []struct {
		name string
		err  error
	}{
		{"ErrInvalidInput", ErrInvalidInput},
		{"ErrNotFound", ErrNotFound},
		{"ErrSessionExpired", ErrSessionExpired},
	}
	for _, s := range sentinels {
		t.Run(s.name, func(t *testing.T) {
			if s.err == nil {
				t.Fatalf("expected %s to be non-nil", s.name)
			}
		})
	}
}

func TestSentinelErrors_Messages(t *testing.T) {
	if ErrInvalidInput.Error() != "invalid input" {
		t.Fatalf("unexpected message: %s", ErrInvalidInput.Error())
	}
	if ErrNotFound.Error() != "not found" {
		t.Fatalf("unexpected message: %s", ErrNotFound.Error())
	}
	if ErrSessionExpired.Error() != "session expired" {
		t.Fatalf("unexpected message: %s", ErrSessionExpired.Error())
	}
}

func TestSentinelErrors_Is(t *testing.T) {
	wrapped := fmt.Errorf("something went wrong: %w", ErrInvalidInput)
	if !errors.Is(wrapped, ErrInvalidInput) {
		t.Fatal("expected errors.Is to match wrapped ErrInvalidInput")
	}
	if errors.Is(wrapped, ErrNotFound) {
		t.Fatal("expected errors.Is to NOT match ErrNotFound")
	}
}

func TestSentinelErrors_Distinct(t *testing.T) {
	if errors.Is(ErrInvalidInput, ErrNotFound) {
		t.Fatal("ErrInvalidInput should not match ErrNotFound")
	}
	if errors.Is(ErrNotFound, ErrSessionExpired) {
		t.Fatal("ErrNotFound should not match ErrSessionExpired")
	}
	if errors.Is(ErrInvalidInput, ErrSessionExpired) {
		t.Fatal("ErrInvalidInput should not match ErrSessionExpired")
	}
}
