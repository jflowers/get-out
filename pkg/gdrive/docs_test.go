package gdrive

import (
	"testing"
)

func TestUtf16Len(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want int64
	}{
		{name: "empty string", s: "", want: 0},
		{name: "ascii", s: "hello", want: 5},
		{name: "ascii with spaces", s: "hello world", want: 11},
		{name: "newline", s: "a\nb", want: 3},
		{name: "emoji (surrogate pair)", s: "😀", want: 2},
		{name: "mixed ascii and emoji", s: "hi 😀!", want: 6},
		{name: "CJK character", s: "中", want: 1},
		{name: "string with multiple emojis", s: "🎉🚀", want: 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := utf16Len(tt.s)
			if got != tt.want {
				t.Errorf("utf16Len(%q) = %d, want %d", tt.s, got, tt.want)
			}
		})
	}
}

func TestGetFieldMask(t *testing.T) {
	tests := []struct {
		name string
		fc   FormattedText
		want string
	}{
		{
			name: "bold only",
			fc:   FormattedText{Bold: true},
			want: "bold",
		},
		{
			name: "italic only",
			fc:   FormattedText{Italic: true},
			want: "italic",
		},
		{
			name: "monospace only",
			fc:   FormattedText{Monospace: true},
			want: "weightedFontFamily",
		},
		{
			name: "bold and italic",
			fc:   FormattedText{Bold: true, Italic: true},
			want: "bold,italic",
		},
		{
			name: "link only",
			fc:   FormattedText{Link: "https://example.com"},
			want: "link",
		},
		{
			name: "all flags",
			fc:   FormattedText{Bold: true, Italic: true, Monospace: true, Link: "https://x.com"},
			want: "bold,italic,weightedFontFamily,link",
		},
		{
			name: "no formatting",
			fc:   FormattedText{Text: "plain"},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getFieldMask(tt.fc)
			if got != tt.want {
				t.Errorf("getFieldMask(%+v) = %q, want %q", tt.fc, got, tt.want)
			}
		})
	}
}
