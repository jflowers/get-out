package chrome

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

// SlackCredentials contains the extracted Slack authentication tokens.
type SlackCredentials struct {
	Token      string // xoxc-... token from localStorage
	Cookie     string // xoxd-... cookie value
	TeamID     string // Team/workspace ID
	TeamDomain string // Team domain (e.g., "mycompany")
}

// localConfigV2 represents the structure of Slack's localStorage data.
type localConfigV2 struct {
	Teams map[string]teamConfig `json:"teams"`
}

type teamConfig struct {
	Token  string `json:"token"`
	ID     string `json:"id"`
	Domain string `json:"domain"`
	Name   string `json:"name"`
}

// ExtractCredentials extracts Slack credentials from the browser session.
// It navigates to a Slack tab and extracts the xoxc token and xoxd cookie.
func (s *Session) ExtractCredentials(ctx context.Context) (*SlackCredentials, error) {
	// Find the Slack tab
	slackTarget, err := s.FindSlackTarget(ctx)
	if err != nil {
		return nil, err
	}

	// Switch to the Slack tab
	tabCtx, cancel := chromedp.NewContext(s.ctx, chromedp.WithTargetID(target.ID(slackTarget.TargetID)))
	defer cancel()

	// Extract token from localStorage
	var localConfigRaw string
	err = chromedp.Run(tabCtx,
		chromedp.Evaluate(`localStorage.getItem('localConfig_v2')`, &localConfigRaw),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to read localStorage: %w", err)
	}

	if localConfigRaw == "" {
		return nil, fmt.Errorf("no Slack config found in localStorage (localConfig_v2 is empty)")
	}

	// Parse the config
	var config localConfigV2
	if err := json.Unmarshal([]byte(localConfigRaw), &config); err != nil {
		return nil, fmt.Errorf("failed to parse localStorage config: %w", err)
	}

	if len(config.Teams) == 0 {
		return nil, fmt.Errorf("no teams found in localStorage config")
	}

	// Get the first team's token (or we could let user specify which workspace)
	var creds SlackCredentials
	for _, team := range config.Teams {
		if team.Token != "" && strings.HasPrefix(team.Token, "xoxc-") {
			creds.Token = team.Token
			creds.TeamID = team.ID
			creds.TeamDomain = team.Domain
			break
		}
	}

	if creds.Token == "" {
		return nil, fmt.Errorf("no valid xoxc token found in localStorage")
	}

	// Extract the d cookie (xoxd-...)
	cookie, err := s.extractSlackCookie(tabCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to extract cookie: %w", err)
	}
	creds.Cookie = cookie

	// Store in session for convenience
	s.Token = creds.Token
	s.Cookie = creds.Cookie

	return &creds, nil
}

// extractSlackCookie extracts the 'd' cookie from Slack domain.
func (s *Session) extractSlackCookie(ctx context.Context) (string, error) {
	var cookies []*network.Cookie

	err := chromedp.Run(ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			cookies, err = network.GetCookies().Do(cdp.WithExecutor(ctx, chromedp.FromContext(ctx).Target))
			return err
		}),
	)
	if err != nil {
		return "", fmt.Errorf("failed to get cookies: %w", err)
	}

	// Look for the 'd' cookie on slack.com domain
	for _, c := range cookies {
		if c.Name == "d" && strings.Contains(c.Domain, "slack.com") {
			return c.Value, nil
		}
	}

	return "", fmt.Errorf("no 'd' cookie found for slack.com")
}

// ExtractCredentialsForTeam extracts credentials for a specific team/workspace.
func (s *Session) ExtractCredentialsForTeam(ctx context.Context, teamDomain string) (*SlackCredentials, error) {
	// Find the Slack tab
	slackTarget, err := s.FindSlackTarget(ctx)
	if err != nil {
		return nil, err
	}

	// Switch to the Slack tab
	tabCtx, cancel := chromedp.NewContext(s.ctx, chromedp.WithTargetID(target.ID(slackTarget.TargetID)))
	defer cancel()

	// Extract token from localStorage
	var localConfigRaw string
	err = chromedp.Run(tabCtx,
		chromedp.Evaluate(`localStorage.getItem('localConfig_v2')`, &localConfigRaw),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to read localStorage: %w", err)
	}

	if localConfigRaw == "" {
		return nil, fmt.Errorf("no Slack config found in localStorage")
	}

	// Parse the config
	var config localConfigV2
	if err := json.Unmarshal([]byte(localConfigRaw), &config); err != nil {
		return nil, fmt.Errorf("failed to parse localStorage config: %w", err)
	}

	// Find the specific team
	var creds SlackCredentials
	for _, team := range config.Teams {
		if team.Domain == teamDomain && strings.HasPrefix(team.Token, "xoxc-") {
			creds.Token = team.Token
			creds.TeamID = team.ID
			creds.TeamDomain = team.Domain
			break
		}
	}

	if creds.Token == "" {
		return nil, fmt.Errorf("no token found for team domain %q", teamDomain)
	}

	// Extract the d cookie
	cookie, err := s.extractSlackCookie(tabCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to extract cookie: %w", err)
	}
	creds.Cookie = cookie

	s.Token = creds.Token
	s.Cookie = creds.Cookie

	return &creds, nil
}

// ListAvailableTeams returns all teams/workspaces available in the browser session.
func (s *Session) ListAvailableTeams(ctx context.Context) ([]TeamInfo, error) {
	slackTarget, err := s.FindSlackTarget(ctx)
	if err != nil {
		return nil, err
	}

	tabCtx, cancel := chromedp.NewContext(s.ctx, chromedp.WithTargetID(target.ID(slackTarget.TargetID)))
	defer cancel()

	var localConfigRaw string
	err = chromedp.Run(tabCtx,
		chromedp.Evaluate(`localStorage.getItem('localConfig_v2')`, &localConfigRaw),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to read localStorage: %w", err)
	}

	if localConfigRaw == "" {
		return nil, nil
	}

	var config localConfigV2
	if err := json.Unmarshal([]byte(localConfigRaw), &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	var teams []TeamInfo
	for _, t := range config.Teams {
		teams = append(teams, TeamInfo{
			ID:       t.ID,
			Domain:   t.Domain,
			Name:     t.Name,
			HasToken: t.Token != "" && strings.HasPrefix(t.Token, "xoxc-"),
		})
	}

	return teams, nil
}

// TeamInfo contains basic information about a Slack workspace.
type TeamInfo struct {
	ID       string
	Domain   string
	Name     string
	HasToken bool
}
