package timeutil

import (
	"testing"
	"time"
)

func TestParseFloatingTime(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantYear  int
		wantMonth time.Month
		wantDay   int
		wantHour  int
		wantMin   int
		wantErr   bool
	}{
		{
			name:      "Floating local (no tz) - canonical format",
			input:     "2026-06-29T15:30:00",
			wantYear:  2026, wantMonth: time.June, wantDay: 29,
			wantHour: 15, wantMin: 30,
		},
		{
			name:      "UTC Z suffix - strips Z correctly",
			input:     "2026-06-29T15:30:00Z",
			wantYear:  2026, wantMonth: time.June, wantDay: 29,
			wantHour: 15, wantMin: 30,
		},
		{
			name:      "RFC3339 with positive offset - strips offset correctly",
			input:     "2026-06-29T15:30:00+03:00",
			wantYear:  2026, wantMonth: time.June, wantDay: 29,
			wantHour: 15, wantMin: 30,
		},
		{
			name:      "RFC3339 with negative offset -03:00",
			input:     "2026-06-29T15:30:00-03:00",
			wantYear:  2026, wantMonth: time.June, wantDay: 29,
			wantHour: 15, wantMin: 30,
		},
		{
			name:      "Milliseconds with Z suffix",
			input:     "2026-06-26T15:52:20.533Z",
			wantYear:  2026, wantMonth: time.June, wantDay: 26,
			wantHour: 15, wantMin: 52,
		},
		{
			name:      "Milliseconds without tz (JS-style)",
			input:     "2026-07-05T12:00:00.000",
			wantYear:  2026, wantMonth: time.July, wantDay: 5,
			wantHour: 12, wantMin: 0,
		},
		{
			name:      "Year boundary - December 31",
			input:     "2026-12-31T23:59:59",
			wantYear:  2026, wantMonth: time.December, wantDay: 31,
			wantHour: 23, wantMin: 59,
		},
		{
			name:      "Year boundary - January 1",
			input:     "2027-01-01T00:00:00",
			wantYear:  2027, wantMonth: time.January, wantDay: 1,
			wantHour: 0, wantMin: 0,
		},
		{
			name:      "Midnight",
			input:     "2026-03-15T00:00:00",
			wantYear:  2026, wantMonth: time.March, wantDay: 15,
			wantHour: 0, wantMin: 0,
		},
		{
			name:    "Completely invalid string",
			input:   "not-a-date",
			wantErr: true,
		},
		{
			name:    "Empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "Partial date",
			input:   "2026-06",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseFloatingTime(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseFloatingTime(%q) expected error, got time %v", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseFloatingTime(%q) unexpected error: %v", tt.input, err)
			}
			if got.Year() != tt.wantYear {
				t.Errorf("Year: got %d, want %d", got.Year(), tt.wantYear)
			}
			if got.Month() != tt.wantMonth {
				t.Errorf("Month: got %v, want %v", got.Month(), tt.wantMonth)
			}
			if got.Day() != tt.wantDay {
				t.Errorf("Day: got %d, want %d", got.Day(), tt.wantDay)
			}
			if got.Hour() != tt.wantHour {
				t.Errorf("Hour: got %d, want %d", got.Hour(), tt.wantHour)
			}
			if got.Minute() != tt.wantMin {
				t.Errorf("Minute: got %d, want %d", got.Minute(), tt.wantMin)
			}
		})
	}
}

// TestParseFloatingTimeNeverShiftsTimezone ensures that regardless of server timezone
// the parsed date always reflects the written date/time literally.
func TestParseFloatingTimeNeverShiftsTimezone(t *testing.T) {
	// Simulate a server running in a very different timezone (UTC+9, Tokyo)
	tokyoTZ, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Skip("Asia/Tokyo timezone not available on this system")
	}

	origLocal := time.Local
	time.Local = tokyoTZ
	defer func() { time.Local = origLocal }()

	inputs := []struct {
		date string
		wantHour int
	}{
		{"2026-06-29T15:30:00", 15},
		{"2026-06-29T15:30:00Z", 15},
		{"2026-06-29T15:30:00-03:00", 15},
		{"2026-06-29T15:30:00+03:00", 15},
	}

	for _, tc := range inputs {
		got, err := ParseFloatingTime(tc.date)
		if err != nil {
			t.Fatalf("ParseFloatingTime(%q) error: %v", tc.date, err)
		}
		if got.Hour() != tc.wantHour {
			t.Errorf("ParseFloatingTime(%q) in Tokyo TZ: Hour = %d, want %d (timezone leaked!)", tc.date, got.Hour(), tc.wantHour)
		}
	}
}
