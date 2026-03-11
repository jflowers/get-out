package chrome

import (
	"testing"
)

func TestIsSlackURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{name: "app.slack.com is Slack", url: "https://app.slack.com/client/T123/C456", want: true},
		{name: "slack.com root is Slack", url: "https://slack.com", want: true},
		{name: "workspace subdomain is Slack", url: "https://myworkspace.slack.com/archives/C123", want: true},
		{name: "google.com is not Slack", url: "https://www.google.com", want: false},
		{name: "empty string is not Slack", url: "", want: false},
		{name: "URL with slack in path only (no slack.com substring)", url: "https://example.com/slack-export", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsSlackURL(tt.url)
			if got != tt.want {
				t.Errorf("IsSlackURL(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}
