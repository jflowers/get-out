package parser

import (
	"strings"
	"testing"

	"github.com/jflowers/get-out/pkg/config"
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

func TestConvertMrkdwn_UserMentions_PersonResolver(t *testing.T) {
	// PersonResolver from people.json — no UserResolver at all
	personResolver := NewPersonResolver(&config.PeopleConfig{
		People: []config.PersonConfig{
			{SlackID: "U123ABC", DisplayName: "Alice"},
			{SlackID: "U456DEF", DisplayName: "Bob", GoogleEmail: "bob@example.com"},
		},
	})

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "resolved from people.json when no UserResolver",
			input: "Hello <@U123ABC>!",
			want:  "Hello @Alice!",
		},
		{
			name:  "unknown user falls back to raw ID",
			input: "Hello <@U999ZZZ>!",
			want:  "Hello @U999ZZZ!",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := ConvertMrkdwnWithLinks(tt.input, nil, nil, personResolver, nil)
			if got != tt.want {
				t.Errorf("ConvertMrkdwnWithLinks() = %q, want %q", got, tt.want)
			}
		})
	}

	// Verify email link is also generated
	got, links := ConvertMrkdwnWithLinks("Hi <@U456DEF>", nil, nil, personResolver, nil)
	if got != "Hi @Bob" {
		t.Errorf("got %q, want %q", got, "Hi @Bob")
	}
	if len(links) != 1 || links[0].URL != "mailto:bob@example.com" {
		t.Errorf("expected mailto link for Bob, got %v", links)
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
			want:  "Check Example Site",
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
			wantFirst: &SlackLink{
				FullURL:   "https://ws.slack.com/archives/C111/p1111111111111111",
				ChannelID: "C111",
				MessageTS: "1111111111.111111",
			},
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
				// Contract assertion: FullURL matches
				if links[0].FullURL != tt.wantFirst.FullURL {
					t.Errorf("FullURL = %q, want %q", links[0].FullURL, tt.wantFirst.FullURL)
				}
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

func TestFindSlackLinks_MultipleLinksSecondElement(t *testing.T) {
	// Contract assertion: verify properties of all returned links, not just the first
	input := "https://ws.slack.com/archives/C111/p1111111111111111 and https://ws.slack.com/archives/C222/p2222222222222222"
	links := FindSlackLinks(input)
	if len(links) != 2 {
		t.Fatalf("expected 2 links, got %d", len(links))
	}
	// Contract assertions on second link
	if links[1].ChannelID != "C222" {
		t.Errorf("links[1].ChannelID = %q, want C222", links[1].ChannelID)
	}
	if links[1].MessageTS != "2222222222.222222" {
		t.Errorf("links[1].MessageTS = %q, want 2222222222.222222", links[1].MessageTS)
	}
	if links[1].FullURL != "https://ws.slack.com/archives/C222/p2222222222222222" {
		t.Errorf("links[1].FullURL = %q, want https://ws.slack.com/archives/C222/p2222222222222222", links[1].FullURL)
	}
	// Contract assertion: links are in order of appearance
	if links[0].StartIndex >= links[1].StartIndex {
		t.Errorf("expected links in order: links[0].StartIndex=%d >= links[1].StartIndex=%d", links[0].StartIndex, links[1].StartIndex)
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
	// Contract assertion: returned string is non-empty
	if got == "" {
		t.Error("FormatTimestamp() returned empty string")
	}
	// Contract assertion: format is "H:MM PM" (7 chars) or "HH:MM PM" (8 chars)
	if len(got) < 7 || len(got) > 8 {
		t.Errorf("FormatTimestamp() = %q, length %d not in [7,8]", got, len(got))
	}
	// Contract assertion: must contain a colon separating hours and minutes
	if !strings.Contains(got, ":") {
		t.Errorf("FormatTimestamp() = %q, missing colon separator", got)
	}
	// Contract assertion: must end with AM or PM
	if got[len(got)-2:] != "AM" && got[len(got)-2:] != "PM" {
		t.Errorf("FormatTimestamp() = %q, does not end with AM or PM", got)
	}
	// Contract assertion: same input always produces same output (deterministic)
	got2 := FormatTimestamp(ts)
	if got != got2 {
		t.Errorf("FormatTimestamp() not deterministic: %q vs %q", got, got2)
	}
}

func TestFormatTimestamp_ZeroTimestamp(t *testing.T) {
	got := FormatTimestamp("0.000000")
	// Contract assertion: zero timestamp produces a valid formatted string
	if got == "" {
		t.Error("FormatTimestamp(0) returned empty string")
	}
	if !strings.Contains(got, ":") {
		t.Errorf("FormatTimestamp(0) = %q, missing colon separator", got)
	}
}

func TestFormatTimestamp_NoFraction(t *testing.T) {
	got := FormatTimestamp("1706745603")
	// Contract assertion: timestamp without fractional part still works
	if got == "" {
		t.Error("FormatTimestamp() returned empty string for non-fractional ts")
	}
	// Should produce same result as with fraction (fraction is ignored)
	gotWithFraction := FormatTimestamp("1706745603.000000")
	if got != gotWithFraction {
		t.Errorf("FormatTimestamp results differ: %q (no fraction) vs %q (with fraction)", got, gotWithFraction)
	}
}

func TestFormatTimestampFull(t *testing.T) {
	ts := "1706745603.123456"
	got := FormatTimestampFull(ts)
	// Contract assertion: returned string is non-empty
	if got == "" {
		t.Error("FormatTimestampFull() returned empty string")
	}
	// Contract assertion: must contain the year 2024
	if !strings.Contains(got, "2024") {
		t.Errorf("FormatTimestampFull() = %q, expected to contain year 2024", got)
	}
	// Contract assertion: must contain the month — Jan or Feb depending on timezone
	// 1706745603 = 2024-02-01 00:00:03 UTC = 2024-01-31 in US timezones
	if !strings.Contains(got, "Jan") && !strings.Contains(got, "Feb") {
		t.Errorf("FormatTimestampFull() = %q, expected to contain month Jan or Feb", got)
	}
	// Contract assertion: format is "Jan D, YYYY H:MM PM" — must contain comma
	if !strings.Contains(got, ",") {
		t.Errorf("FormatTimestampFull() = %q, expected comma in date format", got)
	}
	// Contract assertion: must contain a colon for time
	if !strings.Contains(got, ":") {
		t.Errorf("FormatTimestampFull() = %q, missing colon separator", got)
	}
	// Contract assertion: must end with AM or PM
	if got[len(got)-2:] != "AM" && got[len(got)-2:] != "PM" {
		t.Errorf("FormatTimestampFull() = %q, does not end with AM or PM", got)
	}
	// Contract assertion: same input always produces same output (deterministic)
	got2 := FormatTimestampFull(ts)
	if got != got2 {
		t.Errorf("FormatTimestampFull() not deterministic: %q vs %q", got, got2)
	}
}

func TestFormatTimestampFull_DifferentDate(t *testing.T) {
	// 1609459200 = 2021-01-01 00:00:00 UTC
	ts := "1609459200.000000"
	got := FormatTimestampFull(ts)
	// Contract assertion: must contain year 2020 or 2021 (depends on timezone)
	if !strings.Contains(got, "2021") && !strings.Contains(got, "2020") {
		t.Errorf("FormatTimestampFull(%q) = %q, expected to contain year 2020 or 2021", ts, got)
	}
	// Contract assertion: different timestamp produces different output than first test
	otherGot := FormatTimestampFull("1706745603.123456")
	if got == otherGot {
		t.Errorf("FormatTimestampFull produced same output for different timestamps: %q", got)
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

// ---------------------------------------------------------------------------
// Contract assertions for FindSlackLinks and ReplaceSlackLinks (Task 6.4)
// ---------------------------------------------------------------------------

func TestFindSlackLinks_ContractAssertions(t *testing.T) {
	text := "Check https://team.slack.com/archives/C04KFBJTDJR/p1706745603123456"
	links := FindSlackLinks(text)

	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}

	// Contract: StartIndex and EndIndex mark the link boundaries
	link := links[0]
	if link.StartIndex < 0 {
		t.Error("StartIndex should be >= 0")
	}
	if link.EndIndex <= link.StartIndex {
		t.Error("EndIndex should be > StartIndex")
	}
	// Contract: extracted substring matches the full URL
	extracted := text[link.StartIndex:link.EndIndex]
	if extracted != link.FullURL {
		t.Errorf("text[%d:%d] = %q, want %q", link.StartIndex, link.EndIndex, extracted, link.FullURL)
	}
	// Contract: timestamp is correctly converted from p-format
	if link.MessageTS != "1706745603.123456" {
		t.Errorf("MessageTS = %q, want 1706745603.123456", link.MessageTS)
	}
	if link.ChannelID != "C04KFBJTDJR" {
		t.Errorf("ChannelID = %q, want C04KFBJTDJR", link.ChannelID)
	}
}

func TestReplaceSlackLinks_ContractAssertions(t *testing.T) {
	text := "Link: https://team.slack.com/archives/C123/p1234567890123456"
	got := ReplaceSlackLinks(text, func(channelID, messageTS string) string {
		return "https://docs.example.com/" + channelID
	})

	// Contract: the returned string has the Slack URL replaced
	if got == text {
		t.Error("ReplaceSlackLinks should have replaced the link")
	}
	if got != "Link: https://docs.example.com/C123" {
		t.Errorf("got %q, want %q", got, "Link: https://docs.example.com/C123")
	}
}

func TestReplaceSlackLinks_PartialResolution(t *testing.T) {
	text := "A: https://team.slack.com/archives/C111/p1000000000000001 B: https://team.slack.com/archives/C222/p2000000000000002"
	got := ReplaceSlackLinks(text, func(channelID, messageTS string) string {
		if channelID == "C111" {
			return "https://docs.google.com/document/d/one"
		}
		return "" // C222 not resolved
	})
	want := "A: https://docs.google.com/document/d/one B: https://team.slack.com/archives/C222/p2000000000000002"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// ConvertMrkdwnWithLinks — additional coverage for CRAP reduction
// ---------------------------------------------------------------------------

func TestConvertMrkdwnWithLinks_UserMentionWithGoogleEmail(t *testing.T) {
	// Test the mailto: link annotation path when a PersonResolver has a GoogleEmail
	pr := NewPersonResolver(&config.PeopleConfig{
		People: []config.PersonConfig{
			{SlackID: "U001", DisplayName: "Alice", GoogleEmail: "alice@example.com"},
			{SlackID: "U002", DisplayName: "Bob"}, // no email
		},
	})

	text := "Hey <@U001> and <@U002>"
	got, links := ConvertMrkdwnWithLinks(text, nil, nil, pr, nil)
	if got != "Hey @Alice and @Bob" {
		t.Errorf("got %q, want %q", got, "Hey @Alice and @Bob")
	}

	// Should have exactly one mailto link for Alice
	foundAliceMailto := false
	for _, l := range links {
		if l.Text == "@Alice" && l.URL == "mailto:alice@example.com" {
			foundAliceMailto = true
		}
	}
	if !foundAliceMailto {
		t.Errorf("expected mailto link for @Alice, got links: %+v", links)
	}
}

func TestConvertMrkdwnWithLinks_ChannelMentionWithoutInlineName(t *testing.T) {
	// Test channel mention without inline name, resolved by ChannelResolver
	cr := NewChannelResolver()
	cr.AddChannel("C999", "engineering")

	text := "See <#C999>"
	got, _ := ConvertMrkdwnWithLinks(text, nil, cr, nil, nil)
	if got != "See #engineering" {
		t.Errorf("got %q, want %q", got, "See #engineering")
	}
}

func TestConvertMrkdwnWithLinks_ChannelMentionWithoutResolverOrName(t *testing.T) {
	// Channel mention with no inline name and no resolver → uses raw ID
	text := "See <#C999>"
	got, _ := ConvertMrkdwnWithLinks(text, nil, nil, nil, nil)
	if got != "See #C999" {
		t.Errorf("got %q, want %q", got, "See #C999")
	}
}

func TestConvertMrkdwnWithLinks_SpecialMentions(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "here", input: "<!here>", want: "@here"},
		{name: "channel", input: "<!channel>", want: "@channel"},
		{name: "everyone", input: "<!everyone>", want: "@everyone"},
		{name: "unknown with label", input: "<!subteam|@backend-team>", want: "@backend-team"},
		{name: "unknown without label", input: "<!date>", want: "@date"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := ConvertMrkdwnWithLinks(tt.input, nil, nil, nil, nil)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConvertMrkdwnWithLinks_URLAnnotations(t *testing.T) {
	// URL with display text should produce a link annotation
	text := "Check <https://example.com|Example Site>"
	got, links := ConvertMrkdwnWithLinks(text, nil, nil, nil, nil)
	if got != "Check Example Site" {
		t.Errorf("got %q, want %q", got, "Check Example Site")
	}
	if len(links) != 1 || links[0].URL != "https://example.com" || links[0].Text != "Example Site" {
		t.Errorf("expected URL link annotation, got %+v", links)
	}
}

func TestConvertMrkdwnWithLinks_URLOnlyAnnotation(t *testing.T) {
	// URL without display text should produce a link annotation where text == URL
	text := "Visit <https://example.com/page>"
	got, links := ConvertMrkdwnWithLinks(text, nil, nil, nil, nil)
	if got != "Visit https://example.com/page" {
		t.Errorf("got %q, want %q", got, "Visit https://example.com/page")
	}
	if len(links) != 1 || links[0].URL != "https://example.com/page" || links[0].Text != "https://example.com/page" {
		t.Errorf("expected URL-only link annotation, got %+v", links)
	}
}

func TestConvertMrkdwnWithLinks_SlackLinkResolver(t *testing.T) {
	// Test that SlackLinkResolver replaces Slack archive URLs before other processing
	resolver := func(channelID, messageTS string) string {
		if channelID == "C456" {
			return "https://docs.google.com/document/d/resolved"
		}
		return ""
	}

	text := "See https://myworkspace.slack.com/archives/C456/p1706788800123456 for details"
	got, _ := ConvertMrkdwnWithLinks(text, nil, nil, nil, resolver)
	if got != "See https://docs.google.com/document/d/resolved for details" {
		t.Errorf("got %q, want %q", got, "See https://docs.google.com/document/d/resolved for details")
	}
}

func TestConvertMrkdwnWithLinks_FormattingStripping(t *testing.T) {
	// Exercise the formatting-removal branches of ConvertMrkdwnWithLinks
	// (bold, italic, strikethrough, inline code, code blocks, HTML entities)
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "bold removal", input: "*bold text*", want: "bold text"},
		{name: "italic removal", input: "_italic text_", want: "italic text"},
		{name: "strikethrough removal", input: "~struck text~", want: "struck text"},
		{name: "inline code removal", input: "`inline code`", want: "inline code"},
		{name: "code block removal", input: "```code\nblock```", want: "code\nblock"},
		{name: "mixed formatting", input: "*bold* and _italic_ and ~strike~ and `code`", want: "bold and italic and strike and code"},
		{name: "HTML amp entity", input: "A &amp; B", want: "A & B"},
		{name: "HTML lt/gt entities", input: "&lt;div&gt;", want: "<div>"},
		{name: "HTML quot entity", input: "&quot;hi&quot;", want: "\"hi\""},
		{name: "HTML apos entity", input: "it&#39;s", want: "it's"},
		{name: "HTML nbsp entity", input: "hello&nbsp;world", want: "hello world"},
		{name: "all entities", input: "&lt;&gt;&amp;&quot;&#39;&nbsp;", want: "<>&\"' "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := ConvertMrkdwnWithLinks(tt.input, nil, nil, nil, nil)
			if got != tt.want {
				t.Errorf("ConvertMrkdwnWithLinks(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestConvertMrkdwnWithLinks_EmptyText(t *testing.T) {
	got, links := ConvertMrkdwnWithLinks("", nil, nil, nil, nil)
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
	if len(links) != 0 {
		t.Errorf("expected no links, got %+v", links)
	}
}

func TestConvertMrkdwnWithLinks_NoResolvers(t *testing.T) {
	// All resolvers nil — user mention falls back to raw ID
	text := "<@U999> in <#C999>"
	got, links := ConvertMrkdwnWithLinks(text, nil, nil, nil, nil)
	if got != "@U999 in #C999" {
		t.Errorf("got %q, want %q", got, "@U999 in #C999")
	}
	if len(links) != 0 {
		t.Errorf("expected no links with nil resolvers, got %+v", links)
	}
}

func TestConvertMrkdwnWithLinks_PersonResolverPriority(t *testing.T) {
	// PersonResolver should take priority over UserResolver for @mention names
	ur := NewUserResolver()
	ur.AddUser(&slackapi.User{
		ID:      "U001",
		Name:    "alice",
		Profile: slackapi.UserProfile{DisplayName: "Alice Slack"},
	})

	pr := NewPersonResolver(&config.PeopleConfig{
		People: []config.PersonConfig{
			{SlackID: "U001", DisplayName: "Alice People"},
		},
	})

	text := "Hey <@U001>"
	got, _ := ConvertMrkdwnWithLinks(text, ur, nil, pr, nil)
	if got != "Hey @Alice People" {
		t.Errorf("got %q, want %q (PersonResolver should take priority)", got, "Hey @Alice People")
	}
}

func TestConvertMrkdwnWithLinks_MentionWithInlineDisplayName(t *testing.T) {
	// When the markup has an inline display name, it should be used
	text := "Hey <@U001|inline-name>"
	got, _ := ConvertMrkdwnWithLinks(text, nil, nil, nil, nil)
	if got != "Hey @inline-name" {
		t.Errorf("got %q, want %q", got, "Hey @inline-name")
	}
}

// ---------------------------------------------------------------------------
// Additional edge case coverage for ConvertMrkdwnWithLinks
// ---------------------------------------------------------------------------

func TestConvertMrkdwnWithLinks_UserMentionRegexNoMatch(t *testing.T) {
	// Text with angle brackets that look like mentions but don't match the regex
	text := "<@invalid> and <not-a-mention>"
	got, _ := ConvertMrkdwnWithLinks(text, nil, nil, nil, nil)
	// <@invalid> doesn't match U[A-Z0-9]+ pattern, so it should stay as-is
	// <not-a-mention> is not a URL or mention pattern
	if got != "<@invalid> and <not-a-mention>" {
		t.Errorf("got %q", got)
	}
}

func TestConvertMrkdwnWithLinks_ChannelMentionRegexNoMatch(t *testing.T) {
	// Channel ref that doesn't match C[A-Z0-9]+ pattern
	text := "<#invalid>"
	got, _ := ConvertMrkdwnWithLinks(text, nil, nil, nil, nil)
	if got != "<#invalid>" {
		t.Errorf("got %q", got)
	}
}

func TestConvertMrkdwnWithLinks_HTMLEntitiesCombo(t *testing.T) {
	// Multiple entity types in a single string exercising decodeHTMLEntities
	text := "A&amp;B &lt;C&gt; &quot;D&quot; E&#39;s &nbsp;F"
	got, _ := ConvertMrkdwnWithLinks(text, nil, nil, nil, nil)
	want := "A&B <C> \"D\" E's  F"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestConvertMrkdwnWithLinks_NoEntities(t *testing.T) {
	// Plain text with no entities or formatting — exercises the pass-through
	text := "Just plain text with no special markers"
	got, _ := ConvertMrkdwnWithLinks(text, nil, nil, nil, nil)
	if got != text {
		t.Errorf("got %q, want %q", got, text)
	}
}

func TestConvertMrkdwnWithLinks_UserMentionFallbackToUserResolver(t *testing.T) {
	// PersonResolver returns empty, so UserResolver is used as fallback
	ur := NewUserResolver()
	ur.AddUser(&slackapi.User{
		ID:      "U007",
		Name:    "james",
		Profile: slackapi.UserProfile{DisplayName: "James Bond"},
	})
	pr := NewPersonResolver(&config.PeopleConfig{
		People: []config.PersonConfig{}, // no match for U007
	})

	text := "Hey <@U007>"
	got, _ := ConvertMrkdwnWithLinks(text, ur, nil, pr, nil)
	if got != "Hey @James Bond" {
		t.Errorf("got %q, want %q", got, "Hey @James Bond")
	}
}

func TestConvertMrkdwnWithLinks_UserMentionNoResolversFallsToRawID(t *testing.T) {
	// No inline name, no PersonResolver, no UserResolver — raw ID used
	text := "Hey <@UFALLBACK>"
	got, _ := ConvertMrkdwnWithLinks(text, nil, nil, nil, nil)
	if got != "Hey @UFALLBACK" {
		t.Errorf("got %q, want %q", got, "Hey @UFALLBACK")
	}
}

func TestConvertMrkdwnWithLinks_SlackLinkResolverNil(t *testing.T) {
	// When slackLinkResolver is nil, Slack archive URLs remain unchanged
	text := "See https://workspace.slack.com/archives/C123/p1706745603123456"
	got, _ := ConvertMrkdwnWithLinks(text, nil, nil, nil, nil)
	if got != "See https://workspace.slack.com/archives/C123/p1706745603123456" {
		t.Errorf("got %q", got)
	}
}

func TestConvertMrkdwnWithLinks_NestedFormatting(t *testing.T) {
	// Formatting markers inside other content
	text := "Use *bold* then _italic_ with ~strike~ and `code` together"
	got, _ := ConvertMrkdwnWithLinks(text, nil, nil, nil, nil)
	want := "Use bold then italic with strike and code together"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestConvertMrkdwnWithLinks_CodeBlockMultiline(t *testing.T) {
	// Code block with multiple lines
	text := "```line1\nline2\nline3```"
	got, _ := ConvertMrkdwnWithLinks(text, nil, nil, nil, nil)
	want := "line1\nline2\nline3"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestConvertMrkdwnWithLinks_URLWithSpecialChars(t *testing.T) {
	// URL with query params and fragments
	text := "<https://example.com/path?q=1&r=2#section|Click here>"
	got, links := ConvertMrkdwnWithLinks(text, nil, nil, nil, nil)
	// After HTML entity decode, &amp; would become & but this URL uses raw &
	if got != "Click here" {
		t.Errorf("got %q, want %q", got, "Click here")
	}
	if len(links) != 1 || links[0].URL != "https://example.com/path?q=1&r=2#section" {
		t.Errorf("unexpected links: %+v", links)
	}
}

func TestConvertMrkdwnWithLinks_MultipleAnnotationTypes(t *testing.T) {
	// Combine user mention with email + URL annotations in one message
	pr := NewPersonResolver(&config.PeopleConfig{
		People: []config.PersonConfig{
			{SlackID: "U001", DisplayName: "Alice", GoogleEmail: "alice@example.com"},
		},
	})

	text := "Hey <@U001>, check <https://example.com|this link>"
	got, links := ConvertMrkdwnWithLinks(text, nil, nil, pr, nil)
	if got != "Hey @Alice, check this link" {
		t.Errorf("got %q, want %q", got, "Hey @Alice, check this link")
	}
	if len(links) != 2 {
		t.Fatalf("expected 2 link annotations, got %d: %+v", len(links), links)
	}
}

// ---------------------------------------------------------------------------
// FindSlackLinks / ReplaceSlackLinks — malformed timestamp coverage
// ---------------------------------------------------------------------------

func TestFindSlackLinks_ShortTimestamp(t *testing.T) {
	// Craft a URL where the p-timestamp portion is shorter than 10 digits.
	// The regex requires at least 1 digit in the capture group, but
	// FindSlackLinks should skip links where len(pTimestamp) < 10.
	//
	// The regex p(\d+) matches any digits after 'p', so we need the regex
	// to actually match but with < 10 digits. However the Slack URL format
	// typically has 16+ digits. We test with exactly 9 digits.
	text := "https://ws.slack.com/archives/C123/p123456789"
	links := FindSlackLinks(text)
	// With only 9 digits in p-timestamp, it should be skipped (< 10)
	if len(links) != 0 {
		t.Errorf("expected 0 links for short timestamp, got %d: %+v", len(links), links)
	}
}

func TestReplaceSlackLinks_ShortTimestamp(t *testing.T) {
	// Same malformed short timestamp — ReplaceSlackLinks should keep original
	text := "See https://ws.slack.com/archives/C123/p123456789 for details"
	resolver := func(channelID, messageTS string) string {
		return "https://docs.google.com/replaced"
	}
	got := ReplaceSlackLinks(text, resolver)
	// Should keep the original URL since timestamp is too short
	if got != text {
		t.Errorf("expected original text for short timestamp, got %q", got)
	}
}

func TestFindSlackLinks_ExactlyTenDigitTimestamp(t *testing.T) {
	// 10 digits is the minimum for a valid p-timestamp
	text := "https://ws.slack.com/archives/C123/p1234567890"
	links := FindSlackLinks(text)
	if len(links) != 1 {
		t.Fatalf("expected 1 link for 10-digit timestamp, got %d", len(links))
	}
	// MessageTS should be "1234567890." (10 digits + "." + empty)
	if links[0].MessageTS != "1234567890." {
		t.Errorf("MessageTS = %q, want %q", links[0].MessageTS, "1234567890.")
	}
}
