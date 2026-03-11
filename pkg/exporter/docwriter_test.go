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
