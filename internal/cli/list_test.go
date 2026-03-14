package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/jflowers/get-out/pkg/config"
	"github.com/jflowers/get-out/pkg/models"
)

func TestListCore_NoConversations(t *testing.T) {
	var buf bytes.Buffer
	listCore(&buf, nil, "", "")
	out := buf.String()

	if !strings.Contains(out, "0 total") {
		t.Errorf("expected '0 total' in output, got:\n%s", out)
	}
}

func TestListCore_AllTypes(t *testing.T) {
	convs := []config.ConversationConfig{
		{ID: "D001", Name: "alice", Type: models.ConversationTypeDM, Mode: models.ExportModeAPI},
		{ID: "G001", Name: "team-chat", Type: models.ConversationTypeMPIM, Mode: models.ExportModeBrowser},
		{ID: "C001", Name: "general", Type: models.ConversationTypeChannel, Mode: models.ExportModeAPI},
		{ID: "C002", Name: "secret-ops", Type: models.ConversationTypePrivateChannel, Mode: models.ExportModeAPI, Share: true},
	}

	var buf bytes.Buffer
	listCore(&buf, convs, "", "")
	out := buf.String()

	if !strings.Contains(out, "4 total") {
		t.Errorf("expected '4 total' in output, got:\n%s", out)
	}

	// Verify type group headers appear
	for _, header := range []string{"Direct Messages (1):", "Group Messages (1):", "Channels (1):", "Private Channels (1):"} {
		if !strings.Contains(out, header) {
			t.Errorf("expected header %q in output, got:\n%s", header, out)
		}
	}

	// Verify conversation names appear
	for _, name := range []string{"alice", "team-chat", "general", "secret-ops"} {
		if !strings.Contains(out, name) {
			t.Errorf("expected conversation name %q in output, got:\n%s", name, out)
		}
	}

	// Verify mode labels
	if !strings.Contains(out, "[API]") {
		t.Errorf("expected '[API]' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "[Browser]") {
		t.Errorf("expected '[Browser]' in output, got:\n%s", out)
	}

	// Verify share annotation
	if !strings.Contains(out, "[share]") {
		t.Errorf("expected '[share]' in output, got:\n%s", out)
	}
}

func TestListCore_FilterByType(t *testing.T) {
	convs := []config.ConversationConfig{
		{ID: "D001", Name: "alice", Type: models.ConversationTypeDM, Mode: models.ExportModeAPI},
		{ID: "C001", Name: "general", Type: models.ConversationTypeChannel, Mode: models.ExportModeAPI},
		{ID: "C002", Name: "random", Type: models.ConversationTypeChannel, Mode: models.ExportModeAPI},
	}

	var buf bytes.Buffer
	listCore(&buf, convs, "channel", "")
	out := buf.String()

	if !strings.Contains(out, "2 total") {
		t.Errorf("expected '2 total' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "general") {
		t.Errorf("expected 'general' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "random") {
		t.Errorf("expected 'random' in output, got:\n%s", out)
	}
	if strings.Contains(out, "alice") {
		t.Errorf("did not expect 'alice' in output (filtered by type=channel), got:\n%s", out)
	}
	if strings.Contains(out, "Direct Messages") {
		t.Errorf("did not expect 'Direct Messages' header in output, got:\n%s", out)
	}
}

func TestListCore_FilterByMode(t *testing.T) {
	convs := []config.ConversationConfig{
		{ID: "D001", Name: "alice", Type: models.ConversationTypeDM, Mode: models.ExportModeAPI},
		{ID: "D002", Name: "bob", Type: models.ConversationTypeDM, Mode: models.ExportModeBrowser},
		{ID: "C001", Name: "general", Type: models.ConversationTypeChannel, Mode: models.ExportModeBrowser},
	}

	var buf bytes.Buffer
	listCore(&buf, convs, "", "browser")
	out := buf.String()

	if !strings.Contains(out, "2 total") {
		t.Errorf("expected '2 total' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "bob") {
		t.Errorf("expected 'bob' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "general") {
		t.Errorf("expected 'general' in output, got:\n%s", out)
	}
	if strings.Contains(out, "alice") {
		t.Errorf("did not expect 'alice' in output (filtered by mode=browser), got:\n%s", out)
	}
}

func TestListCore_BothFilters(t *testing.T) {
	convs := []config.ConversationConfig{
		{ID: "D001", Name: "alice", Type: models.ConversationTypeDM, Mode: models.ExportModeAPI},
		{ID: "D002", Name: "bob", Type: models.ConversationTypeDM, Mode: models.ExportModeBrowser},
		{ID: "C001", Name: "general", Type: models.ConversationTypeChannel, Mode: models.ExportModeBrowser},
		{ID: "C002", Name: "dev", Type: models.ConversationTypeChannel, Mode: models.ExportModeAPI},
	}

	var buf bytes.Buffer
	listCore(&buf, convs, "dm", "browser")
	out := buf.String()

	if !strings.Contains(out, "1 total") {
		t.Errorf("expected '1 total' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "bob") {
		t.Errorf("expected 'bob' in output, got:\n%s", out)
	}
	if strings.Contains(out, "alice") {
		t.Errorf("did not expect 'alice' (api mode), got:\n%s", out)
	}
	if strings.Contains(out, "general") {
		t.Errorf("did not expect 'general' (channel type), got:\n%s", out)
	}
	if strings.Contains(out, "dev") {
		t.Errorf("did not expect 'dev' (channel+api), got:\n%s", out)
	}
}
