package cli

import (
	"fmt"
	"testing"
	"time"
)

func TestParseDateFlag(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantTS  string
		wantErr bool
	}{
		{name: "empty returns empty", input: "", wantTS: ""},
		{name: "valid date 2025-01-15", input: "2025-01-15", wantTS: fmt.Sprintf("%d.000000", mustParseUnix(2025, 1, 15))},
		{name: "valid date 2024-02-29 leap day", input: "2024-02-29", wantTS: fmt.Sprintf("%d.000000", mustParseUnix(2024, 2, 29))},
		{name: "invalid format MM-DD-YYYY", input: "01-15-2025", wantErr: true},
		{name: "invalid format plain text", input: "yesterday", wantErr: true},
		{name: "invalid month 13", input: "2025-13-01", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDateFlag(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseDateFlag(%q) expected error, got %q", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Errorf("parseDateFlag(%q) unexpected error: %v", tt.input, err)
				return
			}
			if tt.wantTS != "" && got != tt.wantTS {
				t.Errorf("parseDateFlag(%q) = %q, want %q", tt.input, got, tt.wantTS)
			}
		})
	}
}

func TestTruncateName(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{name: "short name unchanged", input: "general", maxLen: 30, want: "general"},
		{name: "exact length unchanged", input: "hello", maxLen: 5, want: "hello"},
		{name: "truncated with ellipsis", input: "a very long conversation name!", maxLen: 20, want: "a very long conve..."},
		{name: "maxLen <= 3 no ellipsis", input: "hello", maxLen: 3, want: "hel"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateName(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateName(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestSafePreview(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "normal token shows prefix and suffix", input: "xoxc-1234567890abcde-WXYZ", want: "xoxc-1234567890...WXYZ"},
		{name: "short token masked as length", input: "short", want: "[5 chars]"},
		{name: "exactly 19 chars shows preview", input: "1234567890123456789", want: "123456789012345...6789"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := safePreview(tt.input)
			if got != tt.want {
				t.Errorf("safePreview(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// mustParseUnix is a test helper that computes the Unix timestamp for a YYYY-MM-DD date
// using the same UTC interpretation that parseDateFlag uses (time.Parse with "2006-01-02").
func mustParseUnix(year, month, day int) int64 {
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC).Unix()
}
