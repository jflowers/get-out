// Package chrome provides browser automation for connecting to existing
// Chrome/Zen sessions and extracting Slack authentication tokens.
package chrome

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// Session represents a connection to a Chrome browser with an active Slack session.
type Session struct {
	browser *rod.Browser
	page    *rod.Page
	Token   string // xoxc token extracted from localStorage
	Cookie  string // d= cookie value for API requests
}

// Config holds configuration for connecting to Chrome.
type Config struct {
	// DebugURL is the Chrome DevTools Protocol URL (e.g., "ws://127.0.0.1:9222")
	// If empty, will attempt to find a running Chrome instance.
	DebugURL string

	// SlackWorkspace is the Slack workspace URL to navigate to (e.g., "mycompany.slack.com")
	SlackWorkspace string

	// Timeout for operations
	Timeout time.Duration
}

// DefaultConfig returns a config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Timeout: 30 * time.Second,
	}
}

// Connect establishes a connection to an existing Chrome browser session.
// The browser must be started with remote debugging enabled:
//
//	Chrome: --remote-debugging-port=9222
//	Zen:    Similar flag or via settings
func Connect(ctx context.Context, cfg *Config) (*Session, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	var browser *rod.Browser
	var err error

	if cfg.DebugURL != "" {
		// Connect to specified debug URL
		browser = rod.New().ControlURL(cfg.DebugURL)
	} else {
		// Try to find existing Chrome instance
		debugURL, err := findExistingChrome()
		if err != nil {
			return nil, fmt.Errorf("no debug URL provided and could not find running Chrome: %w", err)
		}
		browser = rod.New().ControlURL(debugURL)
	}

	err = browser.Connect()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to browser: %w", err)
	}

	session := &Session{
		browser: browser,
	}

	return session, nil
}

// findExistingChrome attempts to find a Chrome instance with remote debugging enabled.
func findExistingChrome() (string, error) {
	// Try common default port
	u, err := launcher.ResolveURL("")
	if err == nil {
		return u, nil
	}

	// Try explicit localhost:9222
	return "ws://127.0.0.1:9222", nil
}

// FindSlackPage finds an existing Slack tab in the browser.
func (s *Session) FindSlackPage(ctx context.Context, workspace string) error {
	pages, err := s.browser.Pages()
	if err != nil {
		return fmt.Errorf("failed to get browser pages: %w", err)
	}

	for _, page := range pages {
		info, err := page.Info()
		if err != nil {
			continue
		}

		// Look for Slack pages
		if strings.Contains(info.URL, "slack.com") {
			// If workspace specified, check for it
			if workspace != "" && !strings.Contains(info.URL, workspace) {
				continue
			}
			s.page = page
			return nil
		}
	}

	return errors.New("no Slack page found in browser - please open Slack in your browser first")
}

// NavigateToSlack navigates to a Slack workspace if no existing page is found.
func (s *Session) NavigateToSlack(ctx context.Context, workspace string) error {
	if workspace == "" {
		return errors.New("workspace URL required")
	}

	url := workspace
	if !strings.HasPrefix(workspace, "https://") {
		url = "https://" + workspace
	}
	if !strings.HasSuffix(url, ".slack.com") && !strings.Contains(url, "slack.com") {
		url = url + ".slack.com"
	}

	page, err := s.browser.Page(proto.TargetCreateTarget{URL: url})
	if err != nil {
		return fmt.Errorf("failed to create page: %w", err)
	}

	s.page = page
	return page.WaitLoad()
}

// SlackCredentials holds the extracted authentication data.
type SlackCredentials struct {
	Token      string            // xoxc- token
	Cookie     string            // d= cookie value
	TeamID     string            // Team/workspace ID
	UserID     string            // Current user ID
	Enterprise bool              // Whether this is an enterprise workspace
	Headers    map[string]string // Headers to use for API requests
}

// ExtractCredentials extracts authentication tokens from the Slack page.
func (s *Session) ExtractCredentials(ctx context.Context) (*SlackCredentials, error) {
	if s.page == nil {
		return nil, errors.New("no Slack page connected - call FindSlackPage first")
	}

	creds := &SlackCredentials{
		Headers: make(map[string]string),
	}

	// Extract token from localStorage
	token, err := s.extractToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to extract token: %w", err)
	}
	creds.Token = token
	s.Token = token

	// Extract cookies
	cookie, err := s.extractCookie(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to extract cookie: %w", err)
	}
	creds.Cookie = cookie
	s.Cookie = cookie

	// Extract team and user info from boot data
	teamID, userID, isEnterprise, err := s.extractBootData(ctx)
	if err != nil {
		// Non-fatal - we can proceed without this
		fmt.Printf("Warning: could not extract boot data: %v\n", err)
	} else {
		creds.TeamID = teamID
		creds.UserID = userID
		creds.Enterprise = isEnterprise
	}

	// Build headers for API requests
	creds.Headers["Authorization"] = "Bearer " + token
	creds.Headers["Content-Type"] = "application/json; charset=utf-8"
	creds.Headers["Cookie"] = "d=" + cookie

	return creds, nil
}

// extractToken extracts the xoxc token from Slack's localStorage.
func (s *Session) extractToken(ctx context.Context) (string, error) {
	// Slack stores config in localStorage under 'localConfig_v2'
	result, err := s.page.Eval(`() => {
		// Try localConfig_v2 first (newer Slack)
		let config = localStorage.getItem('localConfig_v2');
		if (config) {
			try {
				let parsed = JSON.parse(config);
				// Token might be in different locations depending on Slack version
				if (parsed.teams) {
					for (let teamId in parsed.teams) {
						let team = parsed.teams[teamId];
						if (team.token) return team.token;
					}
				}
			} catch(e) {}
		}
		
		// Try redux store / boot data
		if (window.boot_data && window.boot_data.api_token) {
			return window.boot_data.api_token;
		}
		
		// Try to find token in any localStorage key
		for (let i = 0; i < localStorage.length; i++) {
			let key = localStorage.key(i);
			let value = localStorage.getItem(key);
			if (value && value.includes('xoxc-')) {
				let match = value.match(/xoxc-[a-zA-Z0-9-]+/);
				if (match) return match[0];
			}
		}
		
		return null;
	}`)
	if err != nil {
		return "", fmt.Errorf("failed to execute token extraction: %w", err)
	}

	token := result.Value.Str()
	if token == "" {
		return "", errors.New("could not find xoxc token in localStorage - make sure you're logged into Slack")
	}

	if !strings.HasPrefix(token, "xoxc-") {
		return "", fmt.Errorf("extracted token has unexpected format: %s...", token[:min(10, len(token))])
	}

	return token, nil
}

// extractCookie extracts the 'd' cookie needed for API authentication.
func (s *Session) extractCookie(ctx context.Context) (string, error) {
	cookies, err := s.page.Cookies([]string{})
	if err != nil {
		return "", fmt.Errorf("failed to get cookies: %w", err)
	}

	for _, cookie := range cookies {
		if cookie.Name == "d" && strings.Contains(cookie.Domain, "slack.com") {
			return cookie.Value, nil
		}
	}

	return "", errors.New("could not find 'd' cookie - make sure you're logged into Slack")
}

// extractBootData extracts team/user info from Slack's boot data.
func (s *Session) extractBootData(ctx context.Context) (teamID, userID string, isEnterprise bool, err error) {
	result, err := s.page.Eval(`() => {
		let data = {
			teamId: null,
			userId: null,
			isEnterprise: false
		};
		
		// Try boot_data global
		if (window.boot_data) {
			data.teamId = window.boot_data.team_id;
			data.userId = window.boot_data.user_id;
			data.isEnterprise = window.boot_data.is_enterprise === true || 
			                    window.boot_data.enterprise_id != null;
		}
		
		// Try localStorage config
		let config = localStorage.getItem('localConfig_v2');
		if (config && !data.teamId) {
			try {
				let parsed = JSON.parse(config);
				if (parsed.lastActiveTeamId) {
					data.teamId = parsed.lastActiveTeamId;
				}
				if (parsed.teams && data.teamId && parsed.teams[data.teamId]) {
					data.userId = parsed.teams[data.teamId].user_id;
				}
			} catch(e) {}
		}
		
		return JSON.stringify(data);
	}`)
	if err != nil {
		return "", "", false, err
	}

	var data struct {
		TeamID       string `json:"teamId"`
		UserID       string `json:"userId"`
		IsEnterprise bool   `json:"isEnterprise"`
	}
	if err := json.Unmarshal([]byte(result.Value.Str()), &data); err != nil {
		return "", "", false, err
	}

	return data.TeamID, data.UserID, data.IsEnterprise, nil
}

// GetPage returns the current Slack page for DOM operations.
func (s *Session) GetPage() *rod.Page {
	return s.page
}

// Close closes the browser connection (does not close the browser itself).
func (s *Session) Close() error {
	if s.browser != nil {
		return s.browser.Close()
	}
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
