package chrome

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// DefaultConfig
// ---------------------------------------------------------------------------

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg == nil {
		t.Fatal("DefaultConfig() returned nil")
	}
	if cfg.DebugPort != 9222 {
		t.Errorf("DebugPort = %d, want 9222", cfg.DebugPort)
	}
	if cfg.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want 30s", cfg.Timeout)
	}
}

func TestDefaultConfig_ReturnsNewInstance(t *testing.T) {
	a := DefaultConfig()
	b := DefaultConfig()
	if a == b {
		t.Error("DefaultConfig() should return distinct pointers")
	}
}

// ---------------------------------------------------------------------------
// IsSlackURL (exported)
// ---------------------------------------------------------------------------

func TestIsSlackURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		// --- positive cases ---
		{name: "app.slack.com with path", url: "https://app.slack.com/client/T123/C456", want: true},
		{name: "bare slack.com", url: "https://slack.com", want: true},
		{name: "slack.com with trailing slash", url: "https://slack.com/", want: true},
		{name: "workspace subdomain", url: "https://myworkspace.slack.com/archives/C123", want: true},
		{name: "deeply nested subdomain", url: "https://a.b.c.slack.com/page", want: true},
		{name: "http scheme", url: "http://app.slack.com/foo", want: true},
		{name: "slack.com with port", url: "https://slack.com:443/path", want: true},
		{name: "subdomain with port", url: "https://app.slack.com:8080/client", want: true},
		{name: "slack.com with query", url: "https://slack.com?foo=bar", want: true},
		{name: "slack.com with fragment", url: "https://slack.com#section", want: true},
		{name: "slack.com with userinfo", url: "https://user:pass@slack.com/path", want: true},

		// --- negative cases ---
		{name: "empty string", url: "", want: false},
		{name: "google.com", url: "https://www.google.com", want: false},
		{name: "slack in path only", url: "https://example.com/slack.com", want: false},
		{name: "slack in path segment", url: "https://example.com/slack-export", want: false},
		{name: "not-slack.com domain", url: "https://not-slack.com", want: false},
		{name: "slackfake.com", url: "https://slackfake.com", want: false},
		{name: "fakeslack.com", url: "https://fakeslack.com/path", want: false},
		{name: "myslack.com (no dot prefix)", url: "https://myslack.com", want: false},
		{name: "slack.com as subdomain of another domain", url: "https://slack.com.evil.com", want: false},
		{name: "no scheme bare host", url: "slack.com", want: false},         // net/url puts this in Path, not Host
		{name: "no scheme with path", url: "app.slack.com/foo", want: false}, // same
		{name: "garbage", url: "://not-a-url", want: false},
		{name: "just a colon", url: ":", want: false},
		{name: "slack.com in query only", url: "https://evil.com?redirect=slack.com", want: false},
		{name: "slack.org", url: "https://slack.org", want: false},
		{name: "slack.com.evil.com", url: "https://slack.com.evil.com/phish", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsSlackURL(tt.url)
			if got != tt.want {
				t.Errorf("IsSlackURL(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// isSlackURL (unexported — delegates to IsSlackURL)
// ---------------------------------------------------------------------------

func TestIsSlackURLUnexported(t *testing.T) {
	// The unexported isSlackURL accepts a full URL (same as IsSlackURL).
	// Verify it agrees with the exported version on a few representative inputs.
	samples := []string{
		"https://app.slack.com/client/T1/C2",
		"https://slack.com",
		"https://google.com",
		"",
		"not-a-url",
	}
	for _, s := range samples {
		if isSlackURL(s) != IsSlackURL(s) {
			t.Errorf("isSlackURL(%q) != IsSlackURL(%q)", s, s)
		}
	}
}

// ---------------------------------------------------------------------------
// Session.Close — no-op, but should not panic
// ---------------------------------------------------------------------------

func TestSessionClose_NilFields(t *testing.T) {
	// Close is a deliberate no-op. Verify it doesn't panic even
	// with a zero-value Session (no contexts, no cancel funcs).
	s := &Session{}
	s.Close() // must not panic
}

func TestSessionClose_ZeroValueSession(t *testing.T) {
	var s Session
	s.Close() // must not panic on value receiver scenario (called via pointer)
}

// ---------------------------------------------------------------------------
// Session.Context
// ---------------------------------------------------------------------------

func TestSessionContext_Nil(t *testing.T) {
	s := &Session{}
	if s.Context() != nil {
		t.Error("Context() on zero-value session should return nil")
	}
}

// ---------------------------------------------------------------------------
// Config zero-value behaviour
// ---------------------------------------------------------------------------

func TestConfig_ZeroValue(t *testing.T) {
	var cfg Config
	if cfg.DebugPort != 0 {
		t.Errorf("zero-value DebugPort = %d, want 0", cfg.DebugPort)
	}
	if cfg.Timeout != 0 {
		t.Errorf("zero-value Timeout = %v, want 0", cfg.Timeout)
	}
}

// ---------------------------------------------------------------------------
// SlackCredentials fields
// ---------------------------------------------------------------------------

func TestSlackCredentials_Fields(t *testing.T) {
	c := SlackCredentials{
		Token:      "xoxc-token-value",
		Cookie:     "xoxd-cookie-value",
		TeamID:     "T12345",
		TeamDomain: "myteam",
	}
	if c.Token != "xoxc-token-value" {
		t.Errorf("Token = %q", c.Token)
	}
	if c.Cookie != "xoxd-cookie-value" {
		t.Errorf("Cookie = %q", c.Cookie)
	}
	if c.TeamID != "T12345" {
		t.Errorf("TeamID = %q", c.TeamID)
	}
	if c.TeamDomain != "myteam" {
		t.Errorf("TeamDomain = %q", c.TeamDomain)
	}
}

// ---------------------------------------------------------------------------
// TargetInfo fields
// ---------------------------------------------------------------------------

func TestTargetInfo_Fields(t *testing.T) {
	ti := TargetInfo{
		TargetID: "ABCD1234",
		Type:     "page",
		Title:    "Slack | general",
		URL:      "https://app.slack.com/client/T1/C2",
	}
	if ti.TargetID != "ABCD1234" {
		t.Errorf("TargetID = %q", ti.TargetID)
	}
	if ti.Type != "page" {
		t.Errorf("Type = %q", ti.Type)
	}
	if ti.Title != "Slack | general" {
		t.Errorf("Title = %q", ti.Title)
	}
	if ti.URL != "https://app.slack.com/client/T1/C2" {
		t.Errorf("URL = %q", ti.URL)
	}
}

// ---------------------------------------------------------------------------
// TeamInfo fields
// ---------------------------------------------------------------------------

func TestTeamInfo_Fields(t *testing.T) {
	ti := TeamInfo{
		ID:       "T999",
		Domain:   "acme",
		Name:     "Acme Corp",
		HasToken: true,
	}
	if ti.ID != "T999" {
		t.Errorf("ID = %q", ti.ID)
	}
	if ti.Domain != "acme" {
		t.Errorf("Domain = %q", ti.Domain)
	}
	if ti.Name != "Acme Corp" {
		t.Errorf("Name = %q", ti.Name)
	}
	if !ti.HasToken {
		t.Error("HasToken should be true")
	}
}

// ---------------------------------------------------------------------------
// ListTargets — HTTP-level test using httptest
// ---------------------------------------------------------------------------

func TestListTargets_ParsesCDPResponse(t *testing.T) {
	// Simulate Chrome's /json/list endpoint.
	targets := []cdpTarget{
		{ID: "AAA", Type: "page", Title: "Slack", URL: "https://app.slack.com/client/T1/C2"},
		{ID: "BBB", Type: "page", Title: "Google", URL: "https://www.google.com"},
		{ID: "CCC", Type: "background_page", Title: "Extension", URL: "chrome-extension://xyz/bg.html"},
	}
	body, _ := json.Marshal(targets)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/json/list" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
	defer srv.Close()

	// Extract port from the test server URL.
	var port int
	fmt.Sscanf(srv.URL, "http://127.0.0.1:%d", &port)
	if port == 0 {
		// httptest may bind to [::] — try again.
		fmt.Sscanf(srv.URL, "http://[::1]:%d", &port)
	}

	// We can only use ListTargets if it hits 127.0.0.1:<port>.
	// The method hard-codes 127.0.0.1, so this works when httptest
	// binds to 127.0.0.1, which is the default on most systems.
	// If the test server bound to a different interface we skip.
	if port == 0 {
		t.Skipf("httptest bound to unexpected address %s", srv.URL)
	}

	s := &Session{debugPort: port}
	ctx := t // testing.T embeds context via Deadline, but we need context.Context
	_ = ctx
	result, err := s.ListTargets(t.Context())
	if err != nil {
		t.Fatalf("ListTargets() error: %v", err)
	}

	if len(result) != 3 {
		t.Fatalf("len(result) = %d, want 3", len(result))
	}

	// Spot-check first target.
	if result[0].TargetID != "AAA" {
		t.Errorf("result[0].TargetID = %q, want AAA", result[0].TargetID)
	}
	if result[0].Title != "Slack" {
		t.Errorf("result[0].Title = %q, want Slack", result[0].Title)
	}
	if result[0].URL != "https://app.slack.com/client/T1/C2" {
		t.Errorf("result[0].URL = %q", result[0].URL)
	}
	if result[0].Type != "page" {
		t.Errorf("result[0].Type = %q, want page", result[0].Type)
	}
}

func TestListTargets_EmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("[]"))
	}))
	defer srv.Close()

	var port int
	fmt.Sscanf(srv.URL, "http://127.0.0.1:%d", &port)
	if port == 0 {
		t.Skipf("httptest bound to unexpected address %s", srv.URL)
	}

	s := &Session{debugPort: port}
	result, err := s.ListTargets(t.Context())
	if err != nil {
		t.Fatalf("ListTargets() error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d targets", len(result))
	}
}

func TestListTargets_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("NOT JSON"))
	}))
	defer srv.Close()

	var port int
	fmt.Sscanf(srv.URL, "http://127.0.0.1:%d", &port)
	if port == 0 {
		t.Skipf("httptest bound to unexpected address %s", srv.URL)
	}

	s := &Session{debugPort: port}
	result, err := s.ListTargets(t.Context())
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
	if result != nil {
		t.Error("expected nil result on error")
	}
}

func TestListTargets_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	var port int
	fmt.Sscanf(srv.URL, "http://127.0.0.1:%d", &port)
	if port == 0 {
		t.Skipf("httptest bound to unexpected address %s", srv.URL)
	}

	s := &Session{debugPort: port}
	// The body is a plain-text error message — JSON unmarshal will fail.
	result, err := s.ListTargets(t.Context())
	if err == nil {
		t.Fatal("expected error on 500 response, got nil")
	}
	if result != nil {
		t.Error("expected nil result on error")
	}
}

func TestListTargets_ConnectionRefused(t *testing.T) {
	// Use a port that (almost certainly) has nothing listening.
	s := &Session{debugPort: 19}
	result, err := s.ListTargets(t.Context())
	if err == nil {
		t.Fatal("expected error when nothing is listening, got nil")
	}
	if result != nil {
		t.Error("expected nil result on error")
	}
}

// ---------------------------------------------------------------------------
// FindSlackTarget — uses ListTargets under the hood
// ---------------------------------------------------------------------------

func TestFindSlackTarget_Found(t *testing.T) {
	targets := []cdpTarget{
		{ID: "1", Type: "page", Title: "Google", URL: "https://www.google.com"},
		{ID: "2", Type: "page", Title: "Slack", URL: "https://app.slack.com/client/T1/C2"},
	}
	body, _ := json.Marshal(targets)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
	defer srv.Close()

	var port int
	fmt.Sscanf(srv.URL, "http://127.0.0.1:%d", &port)
	if port == 0 {
		t.Skipf("httptest bound to unexpected address %s", srv.URL)
	}

	s := &Session{debugPort: port}
	target, err := s.FindSlackTarget(t.Context())
	if err != nil {
		t.Fatalf("FindSlackTarget() error: %v", err)
	}
	if target.TargetID != "2" {
		t.Errorf("TargetID = %q, want 2", target.TargetID)
	}
	if target.URL != "https://app.slack.com/client/T1/C2" {
		t.Errorf("URL = %q", target.URL)
	}
}

func TestFindSlackTarget_NotFound(t *testing.T) {
	targets := []cdpTarget{
		{ID: "1", Type: "page", Title: "Google", URL: "https://www.google.com"},
	}
	body, _ := json.Marshal(targets)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
	defer srv.Close()

	var port int
	fmt.Sscanf(srv.URL, "http://127.0.0.1:%d", &port)
	if port == 0 {
		t.Skipf("httptest bound to unexpected address %s", srv.URL)
	}

	s := &Session{debugPort: port}
	target, err := s.FindSlackTarget(t.Context())
	if err == nil {
		t.Fatal("expected error when no Slack tab, got nil")
	}
	if target != nil {
		t.Error("expected nil target on error")
	}
}

func TestFindSlackTarget_IgnoresNonPageTargets(t *testing.T) {
	// A Slack URL on a non-page target type should be skipped.
	targets := []cdpTarget{
		{ID: "1", Type: "background_page", Title: "Slack BG", URL: "https://app.slack.com/service-worker"},
		{ID: "2", Type: "service_worker", Title: "SW", URL: "https://app.slack.com/sw.js"},
	}
	body, _ := json.Marshal(targets)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
	defer srv.Close()

	var port int
	fmt.Sscanf(srv.URL, "http://127.0.0.1:%d", &port)
	if port == 0 {
		t.Skipf("httptest bound to unexpected address %s", srv.URL)
	}

	s := &Session{debugPort: port}
	target, err := s.FindSlackTarget(t.Context())
	if err == nil {
		t.Fatal("expected error — Slack URL on non-page target should be ignored")
	}
	if target != nil {
		t.Error("expected nil target on error")
	}
}
