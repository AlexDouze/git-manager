package cmd

import (
	"strings"
	"testing"
)

func TestCanonicalConfigKey(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "canonical rootDirectory", input: "rootDirectory", want: "rootDirectory"},
		{name: "lowercase rootdirectory", input: "rootdirectory", want: "rootDirectory"},
		{name: "uppercase ROOTDIRECTORY", input: "ROOTDIRECTORY", want: "rootDirectory"},
		{name: "canonical clone.defaultOptions", input: "clone.defaultOptions", want: "clone.defaultOptions"},
		{name: "lowercase clone.defaultoptions", input: "clone.defaultoptions", want: "clone.defaultOptions"},
		{name: "unknown key", input: "bogus", wantErr: true},
		{name: "empty key", input: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := canonicalConfigKey(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("canonicalConfigKey(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if tt.wantErr {
				// The error should list the valid keys to help the user.
				if !strings.Contains(err.Error(), "rootDirectory") {
					t.Errorf("error %q should list valid keys", err.Error())
				}
				return
			}
			if got != tt.want {
				t.Errorf("canonicalConfigKey(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
