package exporter

import (
	"testing"

	"github.com/jflowers/get-out/pkg/config"
	"github.com/jflowers/get-out/pkg/parser"
	"github.com/jflowers/get-out/pkg/slackapi"
)

func TestGroupMessagesByDate(t *testing.T) {
	msgs := []slackapi.Message{
		{TS: "1706745600.000001"}, // 2024-02-01 UTC
		{TS: "1706745601.000002"}, // 2024-02-01 UTC
		{TS: "1706832000.000003"}, // 2024-02-02 UTC
	}

	groups := GroupMessagesByDate(msgs)

	if len(groups) != 2 {
		t.Fatalf("expected 2 date groups, got %d", len(groups))
	}
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
		{TS: "1.0", ThreadTS: ""},          // main
		{TS: "2.0", ThreadTS: "2.0"},       // thread parent (ts == thread_ts)
		{TS: "3.0", ThreadTS: "2.0"},       // thread reply
		{TS: "4.0", ThreadTS: ""},          // main
	}

	main := FilterMainMessages(msgs)

	if len(main) != 3 {
		t.Fatalf("expected 3 main messages, got %d", len(main))
	}
	for _, m := range main {
		if m.ThreadTS != "" && m.TS != m.ThreadTS {
			t.Errorf("non-main message included: TS=%s ThreadTS=%s", m.TS, m.ThreadTS)
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
		ID:    "U002",
		Name:  "botuser",
		IsBot: true,
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
}
