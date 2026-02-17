// Package chrome provides browser automation for connecting to existing
// Chrome/Chromium sessions and extracting Slack authentication tokens.
package chrome

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/chromedp/chromedp"
)

// Session represents a connection to a Chrome browser with an active Slack session.
type Session struct {
	allocCtx    context.Context
	allocCancel context.CancelFunc
	ctx         context.Context
	debugPort   int

	// Extracted credentials
	Token  string // xoxc token from localStorage
	Cookie string // xoxd cookie value
}

// Config holds configuration for connecting to Chrome.
type Config struct {
	// DebugPort is the Chrome DevTools Protocol port (default: 9222)
	DebugPort int

	// Timeout for operations
	Timeout time.Duration
}

// DefaultConfig returns a config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		DebugPort: 9222,
		Timeout:   30 * time.Second,
	}
}

// Connect establishes a connection to an existing Chrome browser session.
// The browser must be started with remote debugging enabled:
//
//	Chrome: --remote-debugging-port=9222
//	Zen:    Similar flag or via browser settings
//
// Example browser launch:
//
//	/Applications/Google\ Chrome.app/Contents/MacOS/Google\ Chrome --remote-debugging-port=9222
func Connect(ctx context.Context, cfg *Config) (*Session, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	debugURL := fmt.Sprintf("ws://127.0.0.1:%d", cfg.DebugPort)

	// IMPORTANT: Use context.Background() for the allocator, NOT the caller's ctx.
	// If we used the caller's ctx (which is cancelled on Ctrl+C), the cancellation
	// would cascade through chromedp and send Target.closeTarget CDP commands,
	// closing the user's browser tabs (including the Slack tab).
	allocCtx, allocCancel := chromedp.NewRemoteAllocator(context.Background(), debugURL)

	// Create browser context — this opens a new blank tab for our use.
	// We intentionally discard the cancel func: calling it would send
	// Target.closeTarget and close the blank tab or attached target.
	browserCtx, _ := chromedp.NewContext(allocCtx)

	// Test connection using the caller's timeout, but don't attach
	// the caller's ctx to the chromedp tree.
	testCtx, testCancel := context.WithTimeout(browserCtx, cfg.Timeout)
	defer testCancel()

	var title string
	err := chromedp.Run(testCtx,
		chromedp.Title(&title),
	)
	if err != nil {
		allocCancel()
		return nil, fmt.Errorf("failed to connect to browser at %s: %w", debugURL, err)
	}

	return &Session{
		allocCtx:    allocCtx,
		allocCancel: allocCancel,
		ctx:         browserCtx,
		debugPort:   cfg.DebugPort,
	}, nil
}

// Close releases resources associated with the session.
// It intentionally does NOT close any browser tabs.
// The allocator and browser contexts are decoupled from the caller's
// context specifically to prevent Ctrl+C from closing tabs.
func (s *Session) Close() {
	// No-op: we intentionally don't cancel any chromedp contexts.
	// Canceling the allocator or browser contexts would send CDP
	// Target.closeTarget commands and close browser tabs.
	// For a CLI process, resources are cleaned up on exit.
}

// Context returns the chromedp context for running actions.
func (s *Session) Context() context.Context {
	return s.ctx
}

// cdpTarget is the JSON structure returned by Chrome's /json/list endpoint.
type cdpTarget struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

// ListTargets returns information about all browser tabs/targets.
// Uses the CDP HTTP endpoint directly, which is more reliable than
// chromedp.Targets() when connected to a remote browser.
func (s *Session) ListTargets(ctx context.Context) ([]TargetInfo, error) {
	listURL := fmt.Sprintf("http://127.0.0.1:%d/json/list", s.debugPort)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, listURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list targets: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var targets []cdpTarget
	if err := json.Unmarshal(body, &targets); err != nil {
		return nil, fmt.Errorf("failed to parse targets: %w", err)
	}

	var result []TargetInfo
	for _, t := range targets {
		result = append(result, TargetInfo{
			TargetID: t.ID,
			Type:     t.Type,
			Title:    t.Title,
			URL:      t.URL,
		})
	}
	return result, nil
}

// TargetInfo contains information about a browser tab.
type TargetInfo struct {
	TargetID string
	Type     string
	Title    string
	URL      string
}

// FindSlackTarget finds a browser tab with Slack loaded.
// It looks for tabs with URLs containing "slack.com".
func (s *Session) FindSlackTarget(ctx context.Context) (*TargetInfo, error) {
	targets, err := s.ListTargets(ctx)
	if err != nil {
		return nil, err
	}

	for _, t := range targets {
		if t.Type == "page" && isSlackURL(t.URL) {
			return &t, nil
		}
	}

	return nil, fmt.Errorf("no Slack tab found in browser")
}

// isSlackURL checks if a URL is a Slack page.
func isSlackURL(url string) bool {
	return len(url) > 0 && (contains(url, "slack.com") || contains(url, "app.slack.com"))
}

// contains is a simple substring check.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
