package git

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ParseHumanDuration parses a human-friendly duration string into a time.Duration.
// Supported units: d (days), w (weeks), m (months = 30 days).
// Examples: "30d", "4w", "1m", "3m"
func ParseHumanDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty duration string")
	}

	unit := s[len(s)-1:]
	valueStr := s[:len(s)-1]

	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return 0, fmt.Errorf("invalid duration value %q: %w", s, err)
	}

	if value <= 0 {
		return 0, fmt.Errorf("duration value must be positive: %q", s)
	}

	switch unit {
	case "d":
		return time.Duration(value) * 24 * time.Hour, nil
	case "w":
		return time.Duration(value) * 7 * 24 * time.Hour, nil
	case "m":
		return time.Duration(value) * 30 * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("unsupported duration unit %q in %q (supported: d, w, m)", unit, s)
	}
}
