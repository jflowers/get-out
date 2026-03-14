package exporter

import (
	"testing"

	"github.com/jflowers/get-out/pkg/config"
	"github.com/jflowers/get-out/pkg/parser"
	"github.com/jflowers/get-out/pkg/slackapi"
)

func TestGroupMessagesByDate(t *testing.T) {
	// Use noon UTC timestamps so the local-time date matches UTC date regardless of timezone offset.
	// 1706788800 = 2024-02-01 12:00:00 UTC
	// 1706875200 = 2024-02-02 12:00:00 UTC
	msgs := []slackapi.Message{
		{TS: "1706788800.000001"}, // 2024-02-01 noon UTC
		{TS: "1706788801.000002"}, // 2024-02-01 noon UTC
		{TS: "1706875200.000003"}, // 2024-02-02 noon UTC
	}

	groups := GroupMessagesByDate(msgs)

	if len(groups) != 2 {
		t.Fatalf("expected 2 date groups, got keys %v", dateKeys(groups))
	}
	// Verify the messages are in the correct date groups.
	feb01, ok := groups["2024-02-01"]
	if !ok {
		t.Errorf("expected group for 2024-02-01, got keys: %v", dateKeys(groups))
	} else if len(feb01) != 2 {
		t.Errorf("2024-02-01 group has %d messages, want 2", len(feb01))
	}
	feb02, ok := groups["2024-02-02"]
	if !ok {
		t.Errorf("expected group for 2024-02-02, got keys: %v", dateKeys(groups))
	} else if len(feb02) != 1 {
		t.Errorf("2024-02-02 group has %d messages, want 1", len(feb02))
	}
}

// dateKeys returns the keys of a date-grouped messages map for error output.
func dateKeys(groups map[string][]slackapi.Message) []string {
	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	return keys
}

func TestSortedDates(t *testing.T) {
	groups := map[string][]slackapi.Message{
		"2024-02-03": {{TS: "1706918400.0"}},
		"2024-02-01": {{TS: "1706745600.0"}},
		"2024-02-02": {{TS: "1706832000.0"}},
	}

	dates := SortedDates(groups)

	if len(dates) != 3 {
		t.Fatalf("expected 3 dates, got %d", len(dates))
	}
	if dates[0] != "2024-02-01" || dates[1] != "2024-02-02" || dates[2] != "2024-02-03" {
		t.Errorf("dates not sorted: %v", dates)
	}
}

func TestFilterMainMessages(t *testing.T) {
	msgs := []slackapi.Message{
		{TS: "1.0", ThreadTS: ""},    // main
		{TS: "2.0", ThreadTS: "2.0"}, // thread parent (ts == thread_ts)
		{TS: "3.0", ThreadTS: "2.0"}, // thread reply
		{TS: "4.0", ThreadTS: ""},    // main
	}

	main := FilterMainMessages(msgs)

	if len(main) != 3 {
		t.Fatalf("expected 3 main messages, got %d", len(main))
	}
	// Verify the specific TSes present: 1.0, 2.0 (thread parent), 4.0 — but not 3.0 (reply).
	wantTSes := map[string]bool{"1.0": true, "2.0": true, "4.0": true}
	for _, m := range main {
		if !wantTSes[m.TS] {
			t.Errorf("unexpected message TS %q in main messages", m.TS)
		}
	}
	// Confirm the reply (3.0) is excluded.
	for _, m := range main {
		if m.TS == "3.0" {
			t.Error("thread reply TS=3.0 should not be in main messages")
		}
	}
}

func TestFilterThreadMessages(t *testing.T) {
	msgs := []slackapi.Message{
		{TS: "1.0", ThreadTS: ""},
		{TS: "2.0", ThreadTS: "2.0"},
		{TS: "2.1", ThreadTS: "2.0"},
		{TS: "2.2", ThreadTS: "2.0"},
		{TS: "3.0", ThreadTS: "3.0"},
	}

	thread := FilterThreadMessages(msgs, "2.0")

	if len(thread) != 3 {
		t.Fatalf("expected 3 thread messages, got %d", len(thread))
	}
}

func TestGetThreadParents(t *testing.T) {
	msgs := []slackapi.Message{
		{TS: "1.0", ReplyCount: 0},
		{TS: "2.0", ReplyCount: 5},
		{TS: "3.0", ReplyCount: 0},
		{TS: "4.0", ReplyCount: 2},
	}

	parents := GetThreadParents(msgs)

	if len(parents) != 2 {
		t.Fatalf("expected 2 thread parents, got %d", len(parents))
	}
	if parents[0].TS != "2.0" || parents[1].TS != "4.0" {
		t.Errorf("wrong parents: %v", parents)
	}
}

// ---------------------------------------------------------------------------
// messageToBlock decomposition integration tests
// ---------------------------------------------------------------------------

func TestMessageToBlock_Decomposed_BasicDelegation(t *testing.T) {
	w := NewDocWriter(nil, nil, nil, nil, nil, nil, nil)
	msg := slackapi.Message{
		User: "U001",
		Text: "Hello world",
		TS:   "1706745603.000000",
	}
	block := w.messageToBlock(nil, "C123", "folder", msg)
	if block.SenderName != "U001" {
		t.Errorf("SenderName = %q, want %q", block.SenderName, "U001")
	}
	if block.Content != "Hello world" {
		t.Errorf("Content = %q, want %q", block.Content, "Hello world")
	}
}

func TestMessageToBlock_Decomposed_AllSections(t *testing.T) {
	// Message with text, attachments, files, and reactions all at once
	// Verifies the decomposed helpers integrate correctly.
	w := NewDocWriter(nil, nil, nil, nil, nil, nil, nil)
	msg := slackapi.Message{
		User: "U001",
		Text: "Main text",
		TS:   "1706745603.000000",
		Attachments: []slackapi.Attachment{
			{Text: "Attached"},
		},
		Files: []slackapi.File{
			{Name: "doc.pdf", Mimetype: "application/pdf"},
		},
		Reactions: []slackapi.Reaction{
			{Name: "heart", Count: 1},
		},
	}
	block := w.messageToBlock(nil, "C123", "folder", msg)
	want := "Main text\n> Attached\n[File: doc.pdf]\nReactions: :heart: (1)"
	if block.Content != want {
		t.Errorf("Content = %q, want %q", block.Content, want)
	}
}

func TestMessageToBlock_Decomposed_ReactionsOnly(t *testing.T) {
	w := NewDocWriter(nil, nil, nil, nil, nil, nil, nil)
	msg := slackapi.Message{
		User: "U001",
		TS:   "1706745603.000000",
		Reactions: []slackapi.Reaction{
			{Name: "thumbsup", Count: 2},
		},
	}
	block := w.messageToBlock(nil, "C123", "folder", msg)
	if block.Content != "Reactions: :thumbsup: (2)" {
		t.Errorf("Content = %q", block.Content)
	}
}

func TestMessageToBlock_Decomposed_AttachmentsOnly(t *testing.T) {
	w := NewDocWriter(nil, nil, nil, nil, nil, nil, nil)
	msg := slackapi.Message{
		User: "U001",
		TS:   "1706745603.000000",
		Attachments: []slackapi.Attachment{
			{Text: "Standalone attachment"},
		},
	}
	block := w.messageToBlock(nil, "C123", "folder", msg)
	if block.Content != "> Standalone attachment" {
		t.Errorf("Content = %q", block.Content)
	}
}

func TestMessageToBlock_Decomposed_FilesOnly(t *testing.T) {
	w := NewDocWriter(nil, nil, nil, nil, nil, nil, nil)
	msg := slackapi.Message{
		User: "U001",
		TS:   "1706745603.000000",
		Files: []slackapi.File{
			{Name: "report.pdf", Mimetype: "application/pdf"},
		},
	}
	block := w.messageToBlock(nil, "C123", "folder", msg)
	if block.Content != "[File: report.pdf]" {
		t.Errorf("Content = %q", block.Content)
	}
}

func TestGetSenderName(t *testing.T) {
	resolver := parser.NewUserResolver()
	resolver.AddUser(&slackapi.User{
		ID:      "U001",
		Name:    "jsmith",
		Profile: slackapi.UserProfile{DisplayName: "John Smith"},
	})
	resolver.AddUser(&slackapi.User{
		ID:      "U002",
		Name:    "botuser",
		IsBot:   true,
		Profile: slackapi.UserProfile{DisplayName: "Deploy Bot"},
	})
	resolver.AddUser(&slackapi.User{
		ID:      "U003",
		Name:    "ex-employee",
		Deleted: true,
		Profile: slackapi.UserProfile{DisplayName: "Former User"},
	})

	// PersonResolver with display names from people.json
	personResolver := parser.NewPersonResolver(&config.PeopleConfig{
		People: []config.PersonConfig{
			{SlackID: "U004", DisplayName: "Alice from People.json"},
			{SlackID: "U001", DisplayName: "John from People.json"},
		},
	})

	w := NewDocWriter(nil, nil, resolver, nil, personResolver, nil, nil)

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
			name: "user in both people.json and resolver - people.json wins",
			msg:  slackapi.Message{User: "U001"},
			want: "John from People.json",
		},
		{
			name: "bot user",
			msg:  slackapi.Message{User: "U002"},
			want: "Deploy Bot [bot]",
		},
		{
			name: "deleted user",
			msg:  slackapi.Message{User: "U003"},
			want: "Former User [deactivated]",
		},
		{
			name: "user only in people.json - resolved from people.json",
			msg:  slackapi.Message{User: "U004"},
			want: "Alice from People.json",
		},
		{
			name: "unknown user ID",
			msg:  slackapi.Message{User: "U999"},
			want: "U999",
		},
		{
			name: "bot ID only",
			msg:  slackapi.Message{BotID: "B123"},
			want: "Bot",
		},
		{
			name: "completely empty",
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
// formatReactions tests
// ---------------------------------------------------------------------------

func TestFormatReactions(t *testing.T) {
	tests := []struct {
		name      string
		reactions []slackapi.Reaction
		want      string
	}{
		{
			name:      "nil reactions",
			reactions: nil,
			want:      "",
		},
		{
			name:      "empty reactions",
			reactions: []slackapi.Reaction{},
			want:      "",
		},
		{
			name: "single reaction",
			reactions: []slackapi.Reaction{
				{Name: "thumbsup", Count: 3},
			},
			want: "Reactions: :thumbsup: (3)",
		},
		{
			name: "multiple reactions",
			reactions: []slackapi.Reaction{
				{Name: "thumbsup", Count: 3},
				{Name: "heart", Count: 1},
				{Name: "fire", Count: 5},
			},
			want: "Reactions: :thumbsup: (3) :heart: (1) :fire: (5)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatReactions(tt.reactions)
			if got != tt.want {
				t.Errorf("formatReactions() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// formatAttachments tests
// ---------------------------------------------------------------------------

func TestFormatAttachments(t *testing.T) {
	tests := []struct {
		name        string
		attachments []slackapi.Attachment
		want        string
	}{
		{
			name:        "nil attachments",
			attachments: nil,
			want:        "",
		},
		{
			name:        "empty attachments",
			attachments: []slackapi.Attachment{},
			want:        "",
		},
		{
			name: "attachment with text only",
			attachments: []slackapi.Attachment{
				{Text: "Some attachment text"},
			},
			want: "> Some attachment text",
		},
		{
			name: "attachment with title and link only",
			attachments: []slackapi.Attachment{
				{Title: "My Link", TitleLink: "https://example.com"},
			},
			want: "[My Link](https://example.com)",
		},
		{
			name: "attachment with both text and title link",
			attachments: []slackapi.Attachment{
				{Text: "Description", Title: "My Link", TitleLink: "https://example.com"},
			},
			want: "> Description\n[My Link](https://example.com)",
		},
		{
			name: "attachment with title but no link is ignored",
			attachments: []slackapi.Attachment{
				{Title: "Title Only"},
			},
			want: "",
		},
		{
			name: "multiple attachments",
			attachments: []slackapi.Attachment{
				{Text: "First"},
				{Text: "Second", Title: "Link", TitleLink: "https://example.com"},
			},
			want: "> First\n> Second\n[Link](https://example.com)",
		},
		{
			name: "attachment with empty text and empty title",
			attachments: []slackapi.Attachment{
				{Color: "green"}, // no text, no title
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatAttachments(tt.attachments)
			if got != tt.want {
				t.Errorf("formatAttachments() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// processMessageFiles tests
// ---------------------------------------------------------------------------

func TestProcessMessageFiles_NoFiles(t *testing.T) {
	w := NewDocWriter(nil, nil, nil, nil, nil, nil, nil)
	text, images := w.processMessageFiles(nil, nil, "folder123")
	if text != "" {
		t.Errorf("expected empty text, got %q", text)
	}
	if len(images) != 0 {
		t.Errorf("expected no images, got %d", len(images))
	}
}

func TestProcessMessageFiles_EmptyFiles(t *testing.T) {
	w := NewDocWriter(nil, nil, nil, nil, nil, nil, nil)
	text, images := w.processMessageFiles(nil, []slackapi.File{}, "folder123")
	if text != "" {
		t.Errorf("expected empty text, got %q", text)
	}
	if len(images) != 0 {
		t.Errorf("expected no images, got %d", len(images))
	}
}

func TestProcessMessageFiles_NonImageFiles(t *testing.T) {
	w := NewDocWriter(nil, nil, nil, nil, nil, nil, nil)
	files := []slackapi.File{
		{Name: "report.pdf", Mimetype: "application/pdf"},
		{Name: "data.csv", Mimetype: "text/csv"},
	}
	text, images := w.processMessageFiles(nil, files, "folder123")
	if text != "[File: report.pdf]\n[File: data.csv]" {
		t.Errorf("got text %q, want %q", text, "[File: report.pdf]\n[File: data.csv]")
	}
	if len(images) != 0 {
		t.Errorf("expected no images for non-image files, got %d", len(images))
	}
}

func TestProcessMessageFiles_ImageWithoutClients(t *testing.T) {
	// Image file but no slackClient/gdrive client — falls through to non-image path
	w := NewDocWriter(nil, nil, nil, nil, nil, nil, nil)
	files := []slackapi.File{
		{Name: "photo.png", Mimetype: "image/png"},
	}
	text, images := w.processMessageFiles(nil, files, "folder123")
	if text != "[File: photo.png]" {
		t.Errorf("got text %q, want %q", text, "[File: photo.png]")
	}
	if len(images) != 0 {
		t.Errorf("expected no images without clients, got %d", len(images))
	}
}

func TestProcessMessageFiles_MixedFiles(t *testing.T) {
	// Mix of image (no clients) and non-image files
	w := NewDocWriter(nil, nil, nil, nil, nil, nil, nil)
	files := []slackapi.File{
		{Name: "photo.png", Mimetype: "image/png"},
		{Name: "doc.txt", Mimetype: "text/plain"},
	}
	text, images := w.processMessageFiles(nil, files, "folder123")
	// Both should appear as text refs since there are no clients
	if text != "[File: photo.png]\n[File: doc.txt]" {
		t.Errorf("got text %q, want %q", text, "[File: photo.png]\n[File: doc.txt]")
	}
	if len(images) != 0 {
		t.Errorf("expected no images, got %d", len(images))
	}
}

func TestFormatMessageTime(t *testing.T) {
	// 1706745603 = some time of day
	got := formatMessageTime("1706745603.000000")
	if got == "" {
		t.Error("formatMessageTime returned empty string")
	}
	// Format contract: must end with AM or PM (e.g., "3:04 PM").
	// The exact value is timezone-dependent.
	if len(got) < 5 {
		t.Errorf("formatMessageTime() = %q, too short to be a valid time", got)
	}
	if got[len(got)-2:] != "AM" && got[len(got)-2:] != "PM" {
		t.Errorf("formatMessageTime() = %q, does not end with AM or PM", got)
	}
}
