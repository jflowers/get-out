package exporter

import (
	"strings"
	"testing"

	"github.com/jflowers/get-out/pkg/config"
	"github.com/jflowers/get-out/pkg/parser"
	"github.com/jflowers/get-out/pkg/slackapi"
)

// ---------------------------------------------------------------------------
// Helper: create a MarkdownWriter with optional resolvers for tests
// ---------------------------------------------------------------------------

func newTestMarkdownWriter() *MarkdownWriter {
	return NewMarkdownWriter(nil, nil, nil)
}

func newTestMarkdownWriterWithResolvers() (*MarkdownWriter, *parser.UserResolver, *parser.PersonResolver) {
	userResolver := parser.NewUserResolver()
	userResolver.AddUser(&slackapi.User{
		ID:      "U001",
		Name:    "alice",
		Profile: slackapi.UserProfile{DisplayName: "Alice"},
	})
	userResolver.AddUser(&slackapi.User{
		ID:      "U002",
		Name:    "bob",
		Profile: slackapi.UserProfile{DisplayName: "Bob"},
	})
	userResolver.AddUser(&slackapi.User{
		ID:      "U003",
		Name:    "botuser",
		IsBot:   true,
		Profile: slackapi.UserProfile{DisplayName: "Deploy Bot"},
	})

	personResolver := parser.NewPersonResolver(&config.PeopleConfig{
		People: []config.PersonConfig{
			{SlackID: "U004", DisplayName: "Charlie from People"},
		},
	})

	channelResolver := parser.NewChannelResolver()

	w := NewMarkdownWriter(userResolver, channelResolver, personResolver)
	return w, userResolver, personResolver
}

// ---------------------------------------------------------------------------
// RenderDailyDoc: frontmatter correctness
// ---------------------------------------------------------------------------

func TestRenderDailyDoc_Frontmatter(t *testing.T) {
	w, _, _ := newTestMarkdownWriterWithResolvers()

	messages := []slackapi.Message{
		{User: "U001", Text: "Hello", TS: "1706788800.000001"},
		{User: "U002", Text: "Hi there", TS: "1706788801.000002"},
	}

	doc, err := w.RenderDailyDoc("general", "channel", "2024-02-01", messages)
	if err != nil {
		t.Fatalf("RenderDailyDoc() error = %v", err)
	}

	content := string(doc)

	// Check frontmatter structure
	if !strings.HasPrefix(content, "---\n") {
		t.Error("document should start with ---")
	}

	mustContain(t, content, "conversation: general")
	mustContain(t, content, "type: channel")
	mustContain(t, content, `date: "2024-02-01"`)
	mustContain(t, content, "participants:")
	mustContain(t, content, "  - Alice")
	mustContain(t, content, "  - Bob")
}

func TestRenderDailyDoc_FrontmatterPrivateChannelType(t *testing.T) {
	w := newTestMarkdownWriter()

	messages := []slackapi.Message{
		{User: "U001", Text: "Secret stuff", TS: "1706788800.000001"},
	}

	doc, err := w.RenderDailyDoc("secret-channel", "private_channel", "2024-02-01", messages)
	if err != nil {
		t.Fatalf("RenderDailyDoc() error = %v", err)
	}

	// type should use canonical ConversationType value with underscore
	mustContain(t, string(doc), "type: private_channel")
}

// ---------------------------------------------------------------------------
// RenderDailyDoc: message ordering
// ---------------------------------------------------------------------------

func TestRenderDailyDoc_MessageOrdering(t *testing.T) {
	w, _, _ := newTestMarkdownWriterWithResolvers()

	// Provide messages out of order
	messages := []slackapi.Message{
		{User: "U002", Text: "Second", TS: "1706788802.000002"},
		{User: "U001", Text: "First", TS: "1706788800.000001"},
		{User: "U001", Text: "Third", TS: "1706788810.000003"},
	}

	doc, err := w.RenderDailyDoc("general", "channel", "2024-02-01", messages)
	if err != nil {
		t.Fatalf("RenderDailyDoc() error = %v", err)
	}

	content := string(doc)
	firstIdx := strings.Index(content, "First")
	secondIdx := strings.Index(content, "Second")
	thirdIdx := strings.Index(content, "Third")

	if firstIdx < 0 || secondIdx < 0 || thirdIdx < 0 {
		t.Fatal("expected all three messages in output")
	}
	if firstIdx > secondIdx || secondIdx > thirdIdx {
		t.Errorf("messages not in chronological order: First@%d, Second@%d, Third@%d", firstIdx, secondIdx, thirdIdx)
	}
}

// ---------------------------------------------------------------------------
// RenderDailyDoc: participants
// ---------------------------------------------------------------------------

func TestRenderDailyDoc_UniqueParticipants(t *testing.T) {
	w, _, _ := newTestMarkdownWriterWithResolvers()

	// Alice sends two messages, Bob sends one
	messages := []slackapi.Message{
		{User: "U001", Text: "msg1", TS: "1706788800.000001"},
		{User: "U002", Text: "msg2", TS: "1706788801.000002"},
		{User: "U001", Text: "msg3", TS: "1706788802.000003"},
	}

	doc, err := w.RenderDailyDoc("general", "channel", "2024-02-01", messages)
	if err != nil {
		t.Fatalf("RenderDailyDoc() error = %v", err)
	}

	content := string(doc)

	// Alice should appear exactly once in participants
	aliceCount := strings.Count(content, "  - Alice")
	if aliceCount != 1 {
		t.Errorf("Alice should appear once in participants, got %d", aliceCount)
	}

	// Participants should be sorted alphabetically: Alice before Bob
	aliceIdx := strings.Index(content, "  - Alice")
	bobIdx := strings.Index(content, "  - Bob")
	if aliceIdx > bobIdx {
		t.Error("participants should be sorted alphabetically (Alice before Bob)")
	}
}

func TestRenderDailyDoc_MultiSender(t *testing.T) {
	w, _, _ := newTestMarkdownWriterWithResolvers()

	messages := []slackapi.Message{
		{User: "U001", Text: "from Alice", TS: "1706788800.000001"},
		{User: "U002", Text: "from Bob", TS: "1706788801.000002"},
		{User: "U004", Text: "from Charlie", TS: "1706788802.000003"},
	}

	doc, err := w.RenderDailyDoc("team", "channel", "2024-02-01", messages)
	if err != nil {
		t.Fatalf("RenderDailyDoc() error = %v", err)
	}

	content := string(doc)
	mustContain(t, content, "  - Alice")
	mustContain(t, content, "  - Bob")
	mustContain(t, content, "  - Charlie from People")
}

// ---------------------------------------------------------------------------
// RenderDailyDoc: empty message slice
// ---------------------------------------------------------------------------

func TestRenderDailyDoc_EmptyMessages(t *testing.T) {
	w := newTestMarkdownWriter()

	doc, err := w.RenderDailyDoc("general", "channel", "2024-02-01", nil)
	if err != nil {
		t.Fatalf("RenderDailyDoc() error = %v", err)
	}

	content := string(doc)

	// Should still have frontmatter
	mustContain(t, content, "---")
	mustContain(t, content, "conversation: general")
	mustContain(t, content, "participants:")

	// Should have no message content after frontmatter closing
	parts := strings.SplitN(content, "---\n", 3)
	if len(parts) < 3 {
		t.Fatal("expected frontmatter delimiters")
	}
	body := strings.TrimSpace(parts[2])
	if body != "" {
		t.Errorf("expected empty body after frontmatter, got %q", body)
	}
}

func TestRenderDailyDoc_EmptySlice(t *testing.T) {
	w := newTestMarkdownWriter()

	doc, err := w.RenderDailyDoc("general", "channel", "2024-02-01", []slackapi.Message{})
	if err != nil {
		t.Fatalf("RenderDailyDoc() error = %v", err)
	}

	content := string(doc)
	mustContain(t, content, "conversation: general")
	mustContain(t, content, "participants:")
}

// ---------------------------------------------------------------------------
// RenderDailyDoc: reactions
// ---------------------------------------------------------------------------

func TestRenderDailyDoc_Reactions(t *testing.T) {
	w := newTestMarkdownWriter()

	messages := []slackapi.Message{
		{
			User: "U001",
			Text: "Great news!",
			TS:   "1706788800.000001",
			Reactions: []slackapi.Reaction{
				{Name: "thumbsup", Count: 3},
				{Name: "heart", Count: 1},
			},
		},
	}

	doc, err := w.RenderDailyDoc("general", "channel", "2024-02-01", messages)
	if err != nil {
		t.Fatalf("RenderDailyDoc() error = %v", err)
	}

	content := string(doc)
	mustContain(t, content, "Reactions: :thumbsup: (3) :heart: (1)")
}

// ---------------------------------------------------------------------------
// RenderDailyDoc: attachments
// ---------------------------------------------------------------------------

func TestRenderDailyDoc_Attachments(t *testing.T) {
	w := newTestMarkdownWriter()

	messages := []slackapi.Message{
		{
			User: "U001",
			Text: "Check this out",
			TS:   "1706788800.000001",
			Attachments: []slackapi.Attachment{
				{Text: "Attachment content here"},
			},
		},
	}

	doc, err := w.RenderDailyDoc("general", "channel", "2024-02-01", messages)
	if err != nil {
		t.Fatalf("RenderDailyDoc() error = %v", err)
	}

	content := string(doc)
	mustContain(t, content, "> Attachment content here")
}

func TestRenderDailyDoc_AttachmentWithTitleLink(t *testing.T) {
	w := newTestMarkdownWriter()

	messages := []slackapi.Message{
		{
			User: "U001",
			Text: "See link",
			TS:   "1706788800.000001",
			Attachments: []slackapi.Attachment{
				{Title: "Example", TitleLink: "https://example.com", Text: "A description"},
			},
		},
	}

	doc, err := w.RenderDailyDoc("general", "channel", "2024-02-01", messages)
	if err != nil {
		t.Fatalf("RenderDailyDoc() error = %v", err)
	}

	content := string(doc)
	mustContain(t, content, "> A description")
	mustContain(t, content, "> [Example](https://example.com)")
}

// ---------------------------------------------------------------------------
// RenderDailyDoc: thread parent marker
// ---------------------------------------------------------------------------

func TestRenderDailyDoc_ThreadParentMarker(t *testing.T) {
	w := newTestMarkdownWriter()

	messages := []slackapi.Message{
		{
			User:       "U001",
			Text:       "Thread parent message",
			TS:         "1706788800.000001",
			ThreadTS:   "1706788800.000001",
			ReplyCount: 5,
		},
	}

	doc, err := w.RenderDailyDoc("general", "channel", "2024-02-01", messages)
	if err != nil {
		t.Fatalf("RenderDailyDoc() error = %v", err)
	}

	mustContain(t, string(doc), "**Thread replies:**")
}

func TestRenderDailyDoc_NoThreadMarkerForNonParent(t *testing.T) {
	w := newTestMarkdownWriter()

	messages := []slackapi.Message{
		{
			User:       "U001",
			Text:       "Regular message",
			TS:         "1706788800.000001",
			ReplyCount: 0,
		},
	}

	doc, err := w.RenderDailyDoc("general", "channel", "2024-02-01", messages)
	if err != nil {
		t.Fatalf("RenderDailyDoc() error = %v", err)
	}

	if strings.Contains(string(doc), "**Thread replies:**") {
		t.Error("should not have thread marker for non-parent message")
	}
}

// ---------------------------------------------------------------------------
// RenderDailyDoc: message format
// ---------------------------------------------------------------------------

func TestRenderDailyDoc_MessageFormat(t *testing.T) {
	w, _, _ := newTestMarkdownWriterWithResolvers()

	messages := []slackapi.Message{
		{User: "U001", Text: "Hello world", TS: "1706788800.000001"},
	}

	doc, err := w.RenderDailyDoc("general", "channel", "2024-02-01", messages)
	if err != nil {
		t.Fatalf("RenderDailyDoc() error = %v", err)
	}

	content := string(doc)

	// Should contain the bold header with time and sender
	// Time is timezone-dependent, so just check the pattern
	mustContain(t, content, "-- Alice**")
	mustContain(t, content, "Hello world")
}

// ---------------------------------------------------------------------------
// RenderDailyDoc: bot sender
// ---------------------------------------------------------------------------

func TestRenderDailyDoc_BotSender(t *testing.T) {
	w := newTestMarkdownWriter()

	messages := []slackapi.Message{
		{Username: "deploy-bot", Text: "Deployed v1.0", TS: "1706788800.000001"},
	}

	doc, err := w.RenderDailyDoc("general", "channel", "2024-02-01", messages)
	if err != nil {
		t.Fatalf("RenderDailyDoc() error = %v", err)
	}

	content := string(doc)
	mustContain(t, content, "deploy-bot [bot]")
	mustContain(t, content, "  - deploy-bot [bot]")
}

// ---------------------------------------------------------------------------
// getSenderName (MarkdownWriter)
// ---------------------------------------------------------------------------

func TestMarkdownWriter_getSenderName(t *testing.T) {
	w, _, _ := newTestMarkdownWriterWithResolvers()

	tests := []struct {
		name string
		msg  slackapi.Message
		want string
	}{
		{
			name: "bot message with username",
			msg:  slackapi.Message{Username: "github-bot"},
			want: "github-bot [bot]",
		},
		{
			name: "user resolved from people.json",
			msg:  slackapi.Message{User: "U004"},
			want: "Charlie from People",
		},
		{
			name: "user resolved from UserResolver",
			msg:  slackapi.Message{User: "U001"},
			want: "Alice",
		},
		{
			name: "bot user via UserResolver",
			msg:  slackapi.Message{User: "U003"},
			want: "Deploy Bot [bot]",
		},
		{
			name: "unknown user falls back to raw ID",
			msg:  slackapi.Message{User: "U999"},
			want: "U999",
		},
		{
			name: "bot ID only",
			msg:  slackapi.Message{BotID: "B123"},
			want: "Bot",
		},
		{
			name: "completely empty message",
			msg:  slackapi.Message{},
			want: "Unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := w.getSenderName(tt.msg)
			if got != tt.want {
				t.Errorf("getSenderName() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// SanitizeDirectoryName
// ---------------------------------------------------------------------------

func TestSanitizeDirectoryName(t *testing.T) {
	tests := []struct {
		name     string
		convType string
		convName string
		want     string
	}{
		{
			name:     "dm type",
			convType: "dm",
			convName: "john-doe",
			want:     "dm-john-doe",
		},
		{
			name:     "mpim maps to group",
			convType: "mpim",
			convName: "team-chat",
			want:     "group-team-chat",
		},
		{
			name:     "channel type",
			convType: "channel",
			convName: "general",
			want:     "channel-general",
		},
		{
			name:     "private_channel maps to private-channel",
			convType: "private_channel",
			convName: "secret",
			want:     "private-channel-secret",
		},
		{
			name:     "spaces converted to hyphens",
			convType: "channel",
			convName: "my cool channel",
			want:     "channel-my-cool-channel",
		},
		{
			name:     "special characters removed",
			convType: "channel",
			convName: "hello@world!123",
			want:     "channel-helloworld123",
		},
		{
			name:     "uppercase to lowercase",
			convType: "channel",
			convName: "MyChannel",
			want:     "channel-mychannel",
		},
		{
			name:     "consecutive hyphens collapsed",
			convType: "channel",
			convName: "foo---bar",
			want:     "channel-foo-bar",
		},
		{
			name:     "special chars creating consecutive hyphens",
			convType: "channel",
			convName: "foo@#$bar",
			want:     "channel-foobar",
		},
		{
			name:     "leading and trailing hyphens trimmed",
			convType: "channel",
			convName: "---hello---",
			want:     "channel-hello",
		},
		{
			name:     "empty name after sanitization",
			convType: "dm",
			convName: "@#$%",
			want:     "dm",
		},
		{
			name:     "empty name string",
			convType: "channel",
			convName: "",
			want:     "channel",
		},
		{
			name:     "truncation to 100 chars",
			convType: "channel",
			convName: strings.Repeat("a", 150),
			want:     "channel-" + strings.Repeat("a", 100),
		},
		{
			name:     "truncation removes trailing hyphen",
			convType: "channel",
			// 99 a's + "-b" = 101 chars; truncated to 100 = 99 a's + "-";
			// trailing hyphen trimmed = 99 a's
			convName: strings.Repeat("a", 99) + "-b",
			want:     "channel-" + strings.Repeat("a", 99),
		},
		{
			name:     "name with only spaces",
			convType: "dm",
			convName: "   ",
			want:     "dm",
		},
		{
			name:     "mixed special chars and valid",
			convType: "mpim",
			convName: "Alice & Bob's Chat!",
			want:     "group-alice-bobs-chat",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeDirectoryName(tt.convType, tt.convName)
			if got != tt.want {
				t.Errorf("SanitizeDirectoryName(%q, %q) = %q, want %q", tt.convType, tt.convName, got, tt.want)
			}
		})
	}
}

// TestSanitizeDirectoryName_TruncationEdge verifies that truncation at exactly
// a hyphen boundary doesn't leave a trailing hyphen.
func TestSanitizeDirectoryName_TruncationEdge(t *testing.T) {
	// Create a name that when sanitized has a hyphen at position 100
	// 99 'a's + " " + "b" = after sanitize: "aaa...a-b" (101 chars)
	name := strings.Repeat("a", 99) + " " + strings.Repeat("b", 5)
	result := SanitizeDirectoryName("channel", name)

	// The sanitized name part should be: 99 a's + "-" + "bbbbb" = 105 chars
	// Truncated to 100: 99 a's + "-" which should have trailing hyphen trimmed
	// So final sanitized part = 99 a's
	want := "channel-" + strings.Repeat("a", 99)
	if result != want {
		t.Errorf("SanitizeDirectoryName() = %q, want %q", result, want)
	}
}

// ---------------------------------------------------------------------------
// Integration: full document with all features
// ---------------------------------------------------------------------------

func TestRenderDailyDoc_FullIntegration(t *testing.T) {
	w, _, _ := newTestMarkdownWriterWithResolvers()

	messages := []slackapi.Message{
		{
			User: "U001",
			Text: "Starting the day",
			TS:   "1706788800.000001",
		},
		{
			User:       "U002",
			Text:       "Let's discuss the plan",
			TS:         "1706788801.000002",
			ThreadTS:   "1706788801.000002",
			ReplyCount: 3,
			Reactions: []slackapi.Reaction{
				{Name: "thumbsup", Count: 2},
			},
			Attachments: []slackapi.Attachment{
				{Text: "Plan document"},
			},
		},
		{
			User: "U004",
			Text: "Sounds good",
			TS:   "1706788802.000003",
		},
	}

	doc, err := w.RenderDailyDoc("engineering", "channel", "2024-02-01", messages)
	if err != nil {
		t.Fatalf("RenderDailyDoc() error = %v", err)
	}

	content := string(doc)

	// Frontmatter
	mustContain(t, content, "conversation: engineering")
	mustContain(t, content, "type: channel")
	mustContain(t, content, `date: "2024-02-01"`)

	// Participants (sorted)
	mustContain(t, content, "  - Alice")
	mustContain(t, content, "  - Bob")
	mustContain(t, content, "  - Charlie from People")

	// Message content
	mustContain(t, content, "Starting the day")
	mustContain(t, content, "Let's discuss the plan")
	mustContain(t, content, "Sounds good")

	// Reactions on Bob's message
	mustContain(t, content, "Reactions: :thumbsup: (2)")

	// Attachment on Bob's message
	mustContain(t, content, "> Plan document")

	// Thread marker on Bob's message
	mustContain(t, content, "**Thread replies:**")
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func mustContain(t *testing.T, content, substr string) {
	t.Helper()
	if !strings.Contains(content, substr) {
		t.Errorf("content does not contain %q\ncontent:\n%s", substr, content)
	}
}
