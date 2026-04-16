## Context

The `setup-browser` command launches Chrome with a hardcoded `https://app.slack.com` URL when no Slack tab is already open (`internal/cli/selfservice.go:709`). The `Settings` struct in `pkg/config/types.go` has no Slack-related fields. User-facing hint text in `doctor` and `setup-browser` also hardcodes this URL in multiple locations.

The existing `IsSlackURL()` function in `pkg/chrome/chrome.go` already accepts any `*.slack.com` subdomain for tab detection, so detection is flexible but the launch URL is not.

## Goals / Non-Goals

### Goals
- Allow users to configure which Slack URL Chrome opens via `settings.json`
- Validate the configured URL to prevent misconfiguration (must be `*.slack.com`)
- Update all user-facing hint text to reflect the configured URL
- Maintain full backward compatibility when the field is absent

### Non-Goals
- Changing the Slack API base URL (`https://slack.com/api`) — that's a different concern
- Adding a CLI flag for one-time URL override — config file is sufficient
- Changing the `IsSlackURL()` tab detection logic — it already handles all subdomains
- Supporting non-Slack URLs — validation enforces `*.slack.com`

## Decisions

### 1. Config field name: `slackWorkspaceUrl`

Use `slackWorkspaceUrl` (camelCase JSON, `SlackWorkspaceURL` Go field). This follows the existing naming convention in `Settings` (e.g., `googleCredentialsFile`, `googleDriveFolderId`). The name clearly communicates that it's a workspace-level URL, not an API endpoint.

### 2. Default value: `https://app.slack.com`

Set in `DefaultSettings()` alongside the existing `LogLevel: "INFO"` default. This preserves current behavior for users who don't set the field. An empty string means "use default" — `LoadSettings()` fills it in.

### 3. Validation reuses `IsSlackURL` logic

The validation function `ValidateSlackURL()` in `pkg/config` will:
- Parse the URL with `net/url.Parse`
- Require `https` scheme (reject `http`)
- Require the host matches `slack.com` or `*.slack.com` (same logic as `chrome.IsSlackURL`)
- Run during `LoadSettings()` if the field is non-empty

This ensures consistency — the same domain rules that detect Slack tabs also govern the configured URL.

### 4. Thread settings through `launchChrome()`

Currently `launchChrome(profilePath string, port int)` takes two args. Add a third parameter for the Slack URL: `launchChrome(profilePath string, port int, slackURL string)`. The caller in `runSetupBrowser` already has access to `settings` — pass `settings.SlackWorkspaceURL` through.

### 5. Replace all hardcoded hint text

Six locations in `selfservice.go` reference `https://app.slack.com` in user-facing strings. All will be updated to use the configured URL from settings. The `evalSlackTargets()` function will need the URL passed in or accessed from the settings context.

### 6. Testability (Constitution Principle IV)

`ValidateSlackURL` is a pure function — testable with table-driven tests covering valid URLs, invalid schemes, non-Slack hosts, and malformed URLs. The `launchChrome` signature change is verified by checking the URL appears in the constructed command args. No external services needed.

## Risks / Trade-offs

### Low risk: Minimal surface area
Only `settings.json` parsing, one function signature change, and string replacements. No new dependencies, no new commands, no behavior changes for existing users.

### Trade-off: HTTPS-only enforcement
Rejecting `http://` URLs means local development proxies can't be used. This is acceptable — Slack itself requires HTTPS, and allowing HTTP would be a security concern.

### Trade-off: No CLI flag
Users must edit `settings.json` to change the URL. This is consistent with how other settings work (e.g., `googleDriveFolderId`). A CLI flag could be added later if needed.
