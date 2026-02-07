package parser

import (
	"testing"

	"github.com/jflowers/get-out/pkg/slackapi"
)

// --- ConvertMrkdwn tests ---

func TestConvertMrkdwn_UserMentions(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		resolver *UserResolver
		want     string
	}{
		{
			name:  "mention with display name in markup",
			input: "Hello <@U123ABC|john>!",
			want:  "Hello @john!",
		},
		{
			name:  "mention without display name, no resolver",
			input: "Hello <@U123ABC>!",
			want:  "Hello @U123ABC!",
		},
		{
			name:  "mention without display name, with resolver",
			input: "Hello <@U123ABC>!",
			resolver: func() *UserResolver {
				r := NewUserResolver()
				r.AddUser(&slackapi.User{
					ID:   "U123ABC",
					Name: "jsmith",
					Profile: slackapi.UserProfile{
						DisplayName: "John Smith",
					},
				})
				return r
			}(),
			want: "Hello @John Smith!",
		},
		{
			name:  "multiple mentions",
			input: "<@U111> and <@U222|bob>",
			want:  "@U111 and @bob",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertMrkdwn(tt.input, tt.resolver, nil)
			if got != tt.want {
				t.Errorf("ConvertMrkdwn() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConvertMrkdwn_ChannelMentions(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		resolver *ChannelResolver
		want     string
	}{
		{
			name:  "channel with name in markup",
			input: "See <#C123ABC|general>",
			want:  "See #general",
		},
		{
			name:  "channel without name, no resolver",
			input: "See <#C123ABC>",
			want:  "See #C123ABC",
		},
		{
			name:  "channel without name, with resolver",
			input: "See <#C123ABC>",
			resolver: func() *ChannelResolver {
				r := NewChannelResolver()
				r.AddChannel("C123ABC", "engineering")
				return r
			}(),
			want: "See #engineering",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertMrkdwn(tt.input, nil, tt.resolver)
			if got != tt.want {
				t.Errorf("ConvertMrkdwn() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConvertMrkdwn_URLs(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "URL with text",
			input: "Check <https://example.com|Example Site>",
			want:  "Check Example Site (https://example.com)",
		},
		{
			name:  "URL without text",
			input: "Visit <https://example.com/page>",
			want:  "Visit https://example.com/page",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertMrkdwn(tt.input, nil, nil)
			if got != tt.want {
				t.Errorf("ConvertMrkdwn() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConvertMrkdwn_SpecialMentions(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "here", input: "<!here>", want: "@here"},
		{name: "channel", input: "<!channel>", want: "@channel"},
		{name: "everyone", input: "<!everyone>", want: "@everyone"},
		{name: "custom with label", input: "<!subteam|@devs>", want: "@devs"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertMrkdwn(tt.input, nil, nil)
			if got != tt.want {
				t.Errorf("ConvertMrkdwn() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConvertMrkdwn_Formatting(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "bold", input: "*hello*", want: "hello"},
		{name: "italic", input: "_hello_", want: "hello"},
		{name: "strikethrough", input: "~hello~", want: "hello"},
		{name: "inline code", input: "`code`", want: "code"},
		{name: "code block", input: "```code block```", want: "code block"},
		{name: "mixed formatting", input: "*bold* and _italic_ and ~strike~", want: "bold and italic and strike"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertMrkdwn(tt.input, nil, nil)
			if got != tt.want {
				t.Errorf("ConvertMrkdwn() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConvertMrkdwn_HTMLEntities(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "ampersand", input: "A &amp; B", want: "A & B"},
		{name: "less than", input: "1 &lt; 2", want: "1 < 2"},
		{name: "greater than", input: "2 &gt; 1", want: "2 > 1"},
		{name: "quote", input: "&quot;hello&quot;", want: "\"hello\""},
		{name: "apostrophe", input: "it&#39;s", want: "it's"},
		{name: "multiple entities", input: "&lt;div&gt; &amp; &quot;stuff&quot;", want: "<div> & \"stuff\""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertMrkdwn(tt.input, nil, nil)
			if got != tt.want {
				t.Errorf("ConvertMrkdwn() = %q, want %q", got, tt.want)
			}
		})
	}
}

// --- Slack link tests ---

func TestFindSlackLinks(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantCount int
		wantFirst *SlackLink
	}{
		{
			name:      "single link",
			input:     "See https://myworkspace.slack.com/archives/C04KFBJTDJR/p1706745603123456",
			wantCount: 1,
			wantFirst: &SlackLink{
				FullURL:   "https://myworkspace.slack.com/archives/C04KFBJTDJR/p1706745603123456",
				ChannelID: "C04KFBJTDJR",
				MessageTS: "1706745603.123456",
			},
		},
		{
			name:      "no links",
			input:     "Just regular text",
			wantCount: 0,
		},
		{
			name:      "multiple links",
			input:     "https://ws.slack.com/archives/C111/p1111111111111111 and https://ws.slack.com/archives/C222/p2222222222222222",
			wantCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			links := FindSlackLinks(tt.input)
			if len(links) != tt.wantCount {
				t.Errorf("FindSlackLinks() returned %d links, want %d", len(links), tt.wantCount)
				return
			}
			if tt.wantFirst != nil && len(links) > 0 {
				if links[0].ChannelID != tt.wantFirst.ChannelID {
					t.Errorf("ChannelID = %q, want %q", links[0].ChannelID, tt.wantFirst.ChannelID)
				}
				if links[0].MessageTS != tt.wantFirst.MessageTS {
					t.Errorf("MessageTS = %q, want %q", links[0].MessageTS, tt.wantFirst.MessageTS)
				}
			}
		})
	}
}

func TestReplaceSlackLinks(t *testing.T) {
	input := "See https://ws.slack.com/archives/C04KFBJTDJR/p1706745603123456 for details"
	resolver := func(channelID, messageTS string) string {
		if channelID == "C04KFBJTDJR" {
			return "https://docs.google.com/document/d/abc123"
		}
		return ""
	}

	got := ReplaceSlackLinks(input, resolver)
	want := "See https://docs.google.com/document/d/abc123 for details"
	if got != want {
		t.Errorf("ReplaceSlackLinks() = %q, want %q", got, want)
	}

	// Test with no-match resolver (should keep original)
	noMatchResolver := func(channelID, messageTS string) string { return "" }
	got2 := ReplaceSlackLinks(input, noMatchResolver)
	if got2 != input {
		t.Errorf("ReplaceSlackLinks() with no match = %q, want original %q", got2, input)
	}
}

// --- Timestamp tests ---

func TestFormatTimestamp(t *testing.T) {
	// 1706745603 = 2024-01-31 at some time
	ts := "1706745603.123456"
	got := FormatTimestamp(ts)
	// Just verify it doesn't panic and returns non-empty
	if got == "" {
		t.Error("FormatTimestamp() returned empty string")
	}
}

func TestFormatTimestampFull(t *testing.T) {
	ts := "1706745603.123456"
	got := FormatTimestampFull(ts)
	if got == "" {
		t.Error("FormatTimestampFull() returned empty string")
	}
	// Should contain a year
	if len(got) < 10 {
		t.Errorf("FormatTimestampFull() = %q, expected longer date string", got)
	}
}

func TestTsToTime(t *testing.T) {
	tests := []struct {
		name    string
		ts      string
		wantSec int64
	}{
		{name: "with fraction", ts: "1706745603.123456", wantSec: 1706745603},
		{name: "without fraction", ts: "1706745603", wantSec: 1706745603},
		{name: "zero", ts: "0.000000", wantSec: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tsToTime(tt.ts)
			if got.Unix() != tt.wantSec {
				t.Errorf("tsToTime(%q).Unix() = %d, want %d", tt.ts, got.Unix(), tt.wantSec)
			}
		})
	}
}
