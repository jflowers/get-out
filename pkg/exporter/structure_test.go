package exporter

import (
	"strings"
	"testing"
)

func TestConversationFolderName(t *testing.T) {
	tests := []struct {
		name     string
		convType string
		convName string
		want     string
	}{
		{name: "DM prefix", convType: "dm", convName: "John Smith", want: "DM - John Smith"},
		{name: "MPIM prefix", convType: "mpim", convName: "team-chat", want: "Group - team-chat"},
		{name: "Channel prefix", convType: "channel", convName: "general", want: "Channel - general"},
		{name: "Private prefix", convType: "private_channel", convName: "secret", want: "Private - secret"},
		{name: "Unknown type default", convType: "unknown", convName: "misc", want: "Chat - misc"},
		{name: "Empty type default", convType: "", convName: "misc", want: "Chat - misc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConversationFolderName(tt.convType, tt.convName)
			if got != tt.want {
				t.Errorf("ConversationFolderName(%q, %q) = %q, want %q", tt.convType, tt.convName, got, tt.want)
			}
		})
	}
}

func TestSanitizeFolderName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "slash replaced with dash", input: "A/B", want: "A-B"},
		{name: "backslash replaced with dash", input: `A\B`, want: "A-B"},
		{name: "colon replaced with dash", input: "A:B", want: "A-B"},
		{name: "asterisk removed", input: "A*B", want: "AB"},
		{name: "question mark removed", input: "A?B", want: "AB"},
		{name: "double quote becomes single", input: `A"B`, want: "A'B"},
		{name: "angle brackets become parens", input: "A<B>C", want: "A(B)C"},
		{name: "pipe replaced with dash", input: "A|B", want: "A-B"},
		{name: "whitespace trimmed", input: "  hello  ", want: "hello"},
		{name: "normal name unchanged", input: "general", want: "general"},
		{name: "length limit at 100 chars", input: strings.Repeat("a", 150), want: strings.Repeat("a", 100)},
		{name: "long name trimmed before limit", input: "  " + strings.Repeat("b", 148) + "  ", want: strings.Repeat("b", 100)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeFolderName(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeFolderName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{name: "short string unchanged", input: "hello", maxLen: 10, want: "hello"},
		{name: "exact length unchanged", input: "hello", maxLen: 5, want: "hello"},
		{name: "truncated with ellipsis", input: "hello world", maxLen: 8, want: "hello..."},
		{name: "maxLen <= 3 truncates without ellipsis", input: "hello", maxLen: 3, want: "hel"},
		{name: "maxLen == 1", input: "hello", maxLen: 1, want: "h"},
		{name: "newlines replaced by spaces", input: "a\nb", maxLen: 10, want: "a b"},
		{name: "tabs replaced by spaces", input: "a\tb", maxLen: 10, want: "a b"},
		{name: "carriage returns replaced", input: "a\rb", maxLen: 10, want: "a b"},
		{name: "leading/trailing whitespace trimmed", input: "  hello  ", maxLen: 20, want: "hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}
