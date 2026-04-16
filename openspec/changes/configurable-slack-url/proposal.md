## Why

When `setup-browser` launches Chrome and no Slack tab is open, it hardcodes `https://app.slack.com` as the URL to navigate to. Many organizations use workspace-specific URLs like `https://mycompany.slack.com` which land users directly in their workspace rather than at the generic Slack landing page. Users currently have no way to configure which Slack URL is opened, forcing them to manually navigate after Chrome launches.

## What Changes

Add a `slackWorkspaceUrl` field to `settings.json` that controls which Slack URL is opened when Chrome launches without an existing Slack tab. The URL is validated to ensure it belongs to `*.slack.com`. All user-facing hint text in `setup-browser` and `doctor` is updated to reflect the configured URL. When the field is not set, behavior defaults to `https://app.slack.com` (fully backward compatible).

## Capabilities

### New Capabilities
- `slackWorkspaceUrl setting`: Users can configure `"slackWorkspaceUrl": "https://mycompany.slack.com"` in `settings.json` to control which Slack URL Chrome opens on launch
- `Slack URL validation`: The configured URL is validated to have an `https` scheme and a `slack.com` or `*.slack.com` host, preventing misconfiguration

### Modified Capabilities
- `setup-browser`: Uses the configured Slack URL when launching Chrome (instead of hardcoded `https://app.slack.com`); hint text and sign-in instructions reflect the configured URL
- `doctor`: Slack tab hint text reflects the configured URL

### Removed Capabilities
- None

## Impact

- **`pkg/config/types.go`**: New `SlackWorkspaceURL` field on `Settings` struct with default value
- **`pkg/config/config.go`**: URL validation during `LoadSettings()`
- **`internal/cli/selfservice.go`**: `launchChrome()` and hint text use configured URL instead of hardcoded string
- **`config/settings.json.example`**: Documents the new field
- **Backward compatibility**: Fully backward compatible. Existing configs without the field get `https://app.slack.com` as the default.

## Constitution Alignment

Assessed against the Unbound Force org constitution.

### I. Autonomous Collaboration

**Assessment**: N/A

This change is internal to the get-out hero's configuration system. It does not affect artifact-based communication between heroes or produce shared outputs.

### II. Composability First

**Assessment**: PASS

The new setting is optional with a sensible default. get-out remains independently installable and usable without configuring this field. No new dependencies are introduced.

### III. Observable Quality

**Assessment**: N/A

This change affects CLI user interaction (which URL Chrome opens). It does not alter machine-parseable outputs or provenance metadata.

### IV. Testability

**Assessment**: PASS

URL validation is a pure function testable in isolation. The `launchChrome` behavior change is testable by verifying the configured URL appears in Chrome launch arguments. No external services are required for testing.
