package cli

import (
	"testing"

	"github.com/jflowers/get-out/pkg/config"
	"github.com/jflowers/get-out/pkg/slackapi"
)

func TestBuildPeopleFromUsers_FiltersBotsAndDeleted(t *testing.T) {
	users := []*slackapi.User{
		{ID: "U001", Name: "alice", Profile: slackapi.UserProfile{Email: "alice@example.com", DisplayName: "Alice"}},
		{ID: "U002", Name: "bot-helper", IsBot: true, Profile: slackapi.UserProfile{DisplayName: "Bot"}},
		{ID: "U003", Name: "app-user", IsAppUser: true, Profile: slackapi.UserProfile{DisplayName: "App"}},
		{ID: "U004", Name: "deleted-user", Deleted: true, Profile: slackapi.UserProfile{DisplayName: "Gone"}},
		{ID: "U005", Name: "bob", Profile: slackapi.UserProfile{Email: "bob@example.com", RealName: "Bob Smith"}},
	}

	result := buildPeopleFromUsers(users)

	if len(result) != 2 {
		t.Fatalf("expected 2 people (filtered bots/deleted), got %d", len(result))
	}

	if result[0].SlackID != "U001" {
		t.Errorf("result[0].SlackID = %q, want %q", result[0].SlackID, "U001")
	}
	if result[0].Email != "alice@example.com" {
		t.Errorf("result[0].Email = %q, want %q", result[0].Email, "alice@example.com")
	}
	if result[0].DisplayName != "Alice" {
		t.Errorf("result[0].DisplayName = %q, want %q", result[0].DisplayName, "Alice")
	}

	if result[1].SlackID != "U005" {
		t.Errorf("result[1].SlackID = %q, want %q", result[1].SlackID, "U005")
	}
	// Bob has no DisplayName, so GetDisplayName falls back to RealName
	if result[1].DisplayName != "Bob Smith" {
		t.Errorf("result[1].DisplayName = %q, want %q", result[1].DisplayName, "Bob Smith")
	}
}

func TestBuildPeopleFromUsers_Empty(t *testing.T) {
	result := buildPeopleFromUsers(nil)
	if result != nil {
		t.Errorf("expected nil for empty input, got %v", result)
	}

	result = buildPeopleFromUsers([]*slackapi.User{})
	if result != nil {
		t.Errorf("expected nil for empty slice, got %v", result)
	}
}

func TestBuildPeopleFromUsers_Normal(t *testing.T) {
	users := []*slackapi.User{
		{ID: "U010", Name: "carol", Profile: slackapi.UserProfile{Email: "carol@example.com", DisplayName: "Carol"}},
		{ID: "U011", Name: "dave", Profile: slackapi.UserProfile{Email: "dave@example.com", DisplayName: "Dave"}},
	}

	result := buildPeopleFromUsers(users)

	if len(result) != 2 {
		t.Fatalf("expected 2 people, got %d", len(result))
	}
	if result[0].SlackID != "U010" || result[1].SlackID != "U011" {
		t.Errorf("unexpected IDs: %q, %q", result[0].SlackID, result[1].SlackID)
	}
}

func TestMergePeople_NoExisting(t *testing.T) {
	newPeople := []config.PersonConfig{
		{SlackID: "U001", DisplayName: "Alice"},
		{SlackID: "U002", DisplayName: "Bob"},
	}

	result := mergePeople(newPeople, nil)

	if len(result) != 2 {
		t.Fatalf("expected 2 people, got %d", len(result))
	}
	if result[0].SlackID != "U001" {
		t.Errorf("result[0].SlackID = %q, want %q", result[0].SlackID, "U001")
	}
	if result[1].SlackID != "U002" {
		t.Errorf("result[1].SlackID = %q, want %q", result[1].SlackID, "U002")
	}
}

func TestMergePeople_WithExisting(t *testing.T) {
	existing := []config.PersonConfig{
		{SlackID: "U001", DisplayName: "Alice (existing)", Email: "alice@old.com"},
	}
	newPeople := []config.PersonConfig{
		{SlackID: "U001", DisplayName: "Alice (new)", Email: "alice@new.com"},
		{SlackID: "U002", DisplayName: "Bob"},
	}

	result := mergePeople(newPeople, existing)

	if len(result) != 2 {
		t.Fatalf("expected 2 people (existing + new non-duplicate), got %d", len(result))
	}

	// Existing entry should be preserved as-is (first in result)
	if result[0].SlackID != "U001" {
		t.Errorf("result[0].SlackID = %q, want %q", result[0].SlackID, "U001")
	}
	if result[0].DisplayName != "Alice (existing)" {
		t.Errorf("existing entry should be preserved, got DisplayName=%q", result[0].DisplayName)
	}
	if result[0].Email != "alice@old.com" {
		t.Errorf("existing entry should keep old email, got %q", result[0].Email)
	}

	// New non-duplicate should be added
	if result[1].SlackID != "U002" {
		t.Errorf("result[1].SlackID = %q, want %q", result[1].SlackID, "U002")
	}
}

func TestMergePeople_AllExisting(t *testing.T) {
	existing := []config.PersonConfig{
		{SlackID: "U001", DisplayName: "Alice"},
		{SlackID: "U002", DisplayName: "Bob"},
	}
	newPeople := []config.PersonConfig{
		{SlackID: "U001", DisplayName: "Alice (updated)"},
		{SlackID: "U002", DisplayName: "Bob (updated)"},
	}

	result := mergePeople(newPeople, existing)

	if len(result) != 2 {
		t.Fatalf("expected 2 people (no new entries), got %d", len(result))
	}

	// All entries should be the existing ones (not overwritten)
	if result[0].DisplayName != "Alice" {
		t.Errorf("result[0] should be existing Alice, got %q", result[0].DisplayName)
	}
	if result[1].DisplayName != "Bob" {
		t.Errorf("result[1] should be existing Bob, got %q", result[1].DisplayName)
	}
}
