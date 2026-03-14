package parser

import (
	"testing"

	"github.com/jflowers/get-out/pkg/config"
)

func TestNewPersonResolver(t *testing.T) {
	people := &config.PeopleConfig{
		People: []config.PersonConfig{
			{SlackID: "U001", DisplayName: "Alice", GoogleEmail: "alice@example.com"},
			{SlackID: "U002", DisplayName: "Bob", GoogleEmail: "bob@example.com"},
			{SlackID: "U003", DisplayName: "NoEmail", GoogleEmail: ""},
		},
	}

	pr := NewPersonResolver(people)

	if pr.Count() != 2 {
		t.Errorf("Count() = %d, want 2 (should skip empty emails)", pr.Count())
	}
	if pr.NameCount() != 3 {
		t.Errorf("NameCount() = %d, want 3 (should include all with display names)", pr.NameCount())
	}
}

func TestPersonResolver_ResolveEmail(t *testing.T) {
	people := &config.PeopleConfig{
		People: []config.PersonConfig{
			{SlackID: "U001", GoogleEmail: "alice@example.com"},
		},
	}
	pr := NewPersonResolver(people)

	tests := []struct {
		name   string
		userID string
		want   string
	}{
		{name: "known user", userID: "U001", want: "alice@example.com"},
		{name: "unknown user", userID: "U999", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pr.ResolveEmail(tt.userID)
			if got != tt.want {
				t.Errorf("ResolveEmail(%q) = %q, want %q", tt.userID, got, tt.want)
			}
		})
	}
}

func TestPersonResolver_ResolveName(t *testing.T) {
	people := &config.PeopleConfig{
		People: []config.PersonConfig{
			{SlackID: "U001", DisplayName: "Alice"},
			{SlackID: "U002", DisplayName: "Bob"},
			{SlackID: "U003", DisplayName: ""},  // no display name
		},
	}
	pr := NewPersonResolver(people)

	tests := []struct {
		name   string
		userID string
		want   string
	}{
		{name: "known user", userID: "U001", want: "Alice"},
		{name: "another known user", userID: "U002", want: "Bob"},
		{name: "user without display name", userID: "U003", want: ""},
		{name: "unknown user", userID: "U999", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pr.ResolveName(tt.userID)
			if got != tt.want {
				t.Errorf("ResolveName(%q) = %q, want %q", tt.userID, got, tt.want)
			}
		})
	}
}

func TestPersonResolver_NilSafe(t *testing.T) {
	var pr *PersonResolver

	if pr.ResolveName("U001") != "" {
		t.Error("nil PersonResolver.ResolveName should return empty string")
	}
	if pr.ResolveEmail("U001") != "" {
		t.Error("nil PersonResolver.ResolveEmail should return empty string")
	}
	if pr.Count() != 0 {
		t.Error("nil PersonResolver.Count should return 0")
	}
	if pr.NameCount() != 0 {
		t.Error("nil PersonResolver.NameCount should return 0")
	}
}

func TestNewPersonResolver_NilConfig(t *testing.T) {
	pr := NewPersonResolver(nil)

	if pr == nil {
		t.Fatal("NewPersonResolver(nil) should not return nil")
	}
	if pr.Count() != 0 {
		t.Errorf("Count() = %d, want 0 for nil config", pr.Count())
	}
}

// ---------------------------------------------------------------------------
// Contract assertions (Task 8.5)
// ---------------------------------------------------------------------------

func TestNewPersonResolver_ContractAssertions(t *testing.T) {
	people := &config.PeopleConfig{
		People: []config.PersonConfig{
			{SlackID: "U001", DisplayName: "Alice", GoogleEmail: "alice@example.com"},
			{SlackID: "U002", DisplayName: "Bob"},
		},
	}

	pr := NewPersonResolver(people)

	// Contract: returned resolver is non-nil
	if pr == nil {
		t.Fatal("NewPersonResolver returned nil")
	}

	// Contract: email count reflects only entries with GoogleEmail set
	if got := pr.Count(); got != 1 {
		t.Errorf("Count() = %d, want 1", got)
	}

	// Contract: name count reflects entries with DisplayName set
	if got := pr.NameCount(); got != 2 {
		t.Errorf("NameCount() = %d, want 2", got)
	}

	// Contract: can resolve names and emails by Slack user ID
	if got := pr.ResolveName("U001"); got != "Alice" {
		t.Errorf("ResolveName(U001) = %q, want Alice", got)
	}
	if got := pr.ResolveEmail("U001"); got != "alice@example.com" {
		t.Errorf("ResolveEmail(U001) = %q, want alice@example.com", got)
	}

	// Contract: missing user returns empty string, not error
	if got := pr.ResolveName("MISSING"); got != "" {
		t.Errorf("ResolveName(MISSING) = %q, want empty", got)
	}
	if got := pr.ResolveEmail("MISSING"); got != "" {
		t.Errorf("ResolveEmail(MISSING) = %q, want empty", got)
	}
}
