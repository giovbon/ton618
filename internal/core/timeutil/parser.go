package timeutil

import (
	"strings"
	"time"
)

// ParseFloatingTime parses an appointment event date string safely, handling both floating time (no tz) and legacy RFC3339 formats.
func ParseFloatingTime(dateStr string) (time.Time, error) {
	cleanStr := dateStr
	if len(cleanStr) > 19 {
		// Clean Z or timezone offset to get the exact written time
		if idx := strings.Index(cleanStr, "Z"); idx > -1 {
			cleanStr = cleanStr[:idx]
		}
		if idx := strings.Index(cleanStr, "+"); idx > 10 {
			cleanStr = cleanStr[:idx]
		}
		if idx := strings.LastIndex(cleanStr, "-"); idx > 10 {
			cleanStr = cleanStr[:idx]
		}
		if len(cleanStr) > 19 {
			cleanStr = cleanStr[:19]
		}
	}

	t, err := time.Parse("2006-01-02T15:04:05", cleanStr)
	if err != nil {
		t, err = time.Parse(time.RFC3339, dateStr)
		if err != nil {
			return time.Time{}, err
		}
	}
	return t, nil
}
