package git

import (
	"testing"
	"time"
)

func TestParseHumanDuration(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected time.Duration
		wantErr  bool
	}{
		{name: "1 day", input: "1d", expected: 24 * time.Hour},
		{name: "30 days", input: "30d", expected: 720 * time.Hour},
		{name: "1 week", input: "1w", expected: 7 * 24 * time.Hour},
		{name: "4 weeks", input: "4w", expected: 4 * 7 * 24 * time.Hour},
		{name: "1 month", input: "1m", expected: 30 * 24 * time.Hour},
		{name: "3 months", input: "3m", expected: 90 * 24 * time.Hour},
		{name: "empty string", input: "", wantErr: true},
		{name: "zero days", input: "0d", wantErr: true},
		{name: "negative days", input: "-1d", wantErr: true},
		{name: "unsupported unit", input: "1x", wantErr: true},
		{name: "non-numeric value", input: "abcd", wantErr: true},
		{name: "whitespace padded", input: "  2w  ", expected: 2 * 7 * 24 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseHumanDuration(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseHumanDuration(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.expected {
				t.Errorf("ParseHumanDuration(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}
