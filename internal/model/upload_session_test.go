package model

import "testing"

func TestUploadStatusValues(t *testing.T) {
	statuses := []UploadStatus{
		StatusCreated,
		StatusInProgress,
		StatusCompleted,
		StatusCancelled,
		StatusFailed,
		StatusExpired,
	}
	for _, status := range statuses {
		if status == "" {
			t.Fatalf("expected status to be non-empty")
		}
	}
}
