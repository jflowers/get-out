package parser

import (
	"testing"

	"github.com/jflowers/get-out/pkg/config"
)

func TestConvertMrkdwnToMarkdown(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		userResolver    *UserResolver
		channelResolver *ChannelResolver
		personResolver  *PersonResolver
		want            string
	}{
		// --- Empty / plain text ---
		{
			name:  "empty input",
			input: "",
			want:  "",
		},
		{
			name:  "plain text with no formatting",
			input: "Hello, this is a plain message.",
			want:  "Hello, this is a plain message.",
		},

		// --- Bold ---
		{
			name:  "bold",
			input: "*bold text*",
			want:  "**bold text**",
		},

		// --- Italic ---
		{
			name:  "italic",
			input: "_italic text_",
			want:  "*italic text*",
		},

		// --- Strikethrough ---
		{
			name:  "strikethrough",
			input: "~strikethrough~",
			want:  "~~strikethrough~~",
		},

		// --- Inline code ---
		{
			name:  "inline code preserved",
			input: "`inline code`",
			want:  "`inline code`",
		},

		// --- Code block ---
		{
			name:  "code block becomes fenced",
			input: "```code block```",
			want:  "\n```\ncode block\n```\n",
		},

		// --- URL with display text ---
		{
			name:  "URL with display text becomes markdown link",
			input: "<https://example.com|Example Site>",
			want:  "[Example Site](https://example.com)",
		},

		// --- URL without display text ---
		{
			name:  "URL without display text becomes autolink",
			input: "<https://example.com/page>",
			want:  "<https://example.com/page>",
		},

		// --- User mentions ---
		{
			name:  "user mention with inline display name",
			input: "<@U123ABC|john>",
			want:  "@john",
		},
		{
			name:  "user mention without display name, no resolvers",
			input: "<@U123ABC>",
			want:  "@U123ABC",
		},
		{
			name:  "user mention resolved by person resolver",
			input: "<@U001>",
			personResolver: NewPersonResolver(&config.PeopleConfig{
				People: []config.PersonConfig{
					{SlackID: "U001", DisplayName: "Alice"},
				},
			}),
			want: "@Alice",
		},

		// --- Channel mentions ---
		{
			name:  "channel mention with inline name",
			input: "<#C12345|channel-name>",
			want:  "#channel-name",
		},
		{
			name:  "channel mention without name, no resolver",
			input: "<#C12345>",
			want:  "#C12345",
		},
		{
			name:  "channel mention resolved by channel resolver",
			input: "<#C12345>",
			channelResolver: func() *ChannelResolver {
				r := NewChannelResolver()
				r.AddChannel("C12345", "engineering")
				return r
			}(),
			want: "#engineering",
		},

		// --- Special mentions ---
		{
			name:  "here mention",
			input: "<!here>",
			want:  "@here",
		},
		{
			name:  "channel special mention",
			input: "<!channel>",
			want:  "@channel",
		},
		{
			name:  "everyone mention",
			input: "<!everyone>",
			want:  "@everyone",
		},
		{
			name:  "unknown special mention without display",
			input: "<!subteam>",
			want:  "@subteam",
		},
		{
			name:  "unknown special mention with display",
			input: "<!subteam|@engineering>",
			want:  "@engineering",
		},

		// --- HTML entity decoding ---
		{
			name:  "HTML entities decoded",
			input: "A &amp; B &lt; C &gt; D",
			want:  "A & B < C > D",
		},

		// --- Formatting inside code spans must NOT be converted ---
		{
			name:  "formatting inside inline code preserved",
			input: "`*not bold*`",
			want:  "`*not bold*`",
		},
		{
			name:  "formatting inside code block preserved",
			input: "```*not bold* and _not italic_```",
			want:  "\n```\n*not bold* and _not italic_\n```\n",
		},

		// --- Mixed formatting ---
		{
			name:  "multiple formatting types in one message",
			input: "*bold* and _italic_ and ~strike~ and `code`",
			want:  "**bold** and *italic* and ~~strike~~ and `code`",
		},

		// --- Nested formatting ---
		{
			name:  "bold with italic inside",
			input: "*bold with _italic_ inside*",
			want:  "**bold with *italic* inside**",
		},

		// --- Complex message combining multiple types ---
		{
			name:  "complex message with mentions, URLs, and formatting",
			input: "Hey <!here>, check <https://example.com|this link> for *important* info",
			want:  "Hey @here, check [this link](https://example.com) for **important** info",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertMrkdwnToMarkdown(tt.input, tt.userResolver, tt.channelResolver, tt.personResolver)
			if got != tt.want {
				t.Errorf("ConvertMrkdwnToMarkdown(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
