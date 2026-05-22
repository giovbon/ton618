package capture

import (
	"testing"
	"time"
)

func TestFormatCaptureTimestamp_IncludesTimezone(t *testing.T) {
	loc, err := time.LoadLocation("UTC")
	if err != nil {
		t.Fatal(err)
	}

	timestamp := formatCaptureTimestamp(time.Date(2026, 5, 22, 12, 0, 0, 0, loc))
	if got, want := timestamp, "2026-05-22 12:00:00 UTC"; got != want {
		t.Fatalf("expected timestamp %q, got %q", want, got)
	}
}
