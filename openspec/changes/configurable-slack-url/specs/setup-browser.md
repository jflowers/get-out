## ADDED Requirements

### Requirement: Configurable Slack Workspace URL

The application MUST support a `slackWorkspaceUrl` field in `settings.json` that specifies which Slack URL to open when Chrome launches without an existing Slack tab.

The field MUST default to `https://app.slack.com` when not specified or empty.

#### Scenario: User configures a workspace-specific URL
- **GIVEN** `settings.json` contains `"slackWorkspaceUrl": "https://mycompany.slack.com"`
- **WHEN** `setup-browser` launches Chrome and no Slack tab is open
- **THEN** Chrome MUST open `https://mycompany.slack.com` as the initial URL

#### Scenario: No URL configured (backward compatibility)
- **GIVEN** `settings.json` does not contain a `slackWorkspaceUrl` field
- **WHEN** `setup-browser` launches Chrome and no Slack tab is open
- **THEN** Chrome MUST open `https://app.slack.com` as the initial URL

#### Scenario: Empty URL configured
- **GIVEN** `settings.json` contains `"slackWorkspaceUrl": ""`
- **WHEN** settings are loaded
- **THEN** the application MUST use the default `https://app.slack.com`

### Requirement: Slack URL Validation

The application MUST validate the `slackWorkspaceUrl` field when loading settings.

The URL MUST have an `https` scheme. The URL host MUST be `slack.com` or a subdomain of `slack.com` (matching `*.slack.com`).

#### Scenario: Valid workspace URL accepted
- **GIVEN** `settings.json` contains `"slackWorkspaceUrl": "https://mycompany.slack.com"`
- **WHEN** settings are loaded
- **THEN** the URL MUST be accepted without error

#### Scenario: Valid base URL accepted
- **GIVEN** `settings.json` contains `"slackWorkspaceUrl": "https://app.slack.com"`
- **WHEN** settings are loaded
- **THEN** the URL MUST be accepted without error

#### Scenario: Non-HTTPS URL rejected
- **GIVEN** `settings.json` contains `"slackWorkspaceUrl": "http://mycompany.slack.com"`
- **WHEN** settings are loaded
- **THEN** the application MUST return a validation error indicating HTTPS is required

#### Scenario: Non-Slack URL rejected
- **GIVEN** `settings.json` contains `"slackWorkspaceUrl": "https://example.com"`
- **WHEN** settings are loaded
- **THEN** the application MUST return a validation error indicating the URL must be a slack.com domain

#### Scenario: Malformed URL rejected
- **GIVEN** `settings.json` contains `"slackWorkspaceUrl": "not-a-url"`
- **WHEN** settings are loaded
- **THEN** the application MUST return a validation error

### Requirement: Dynamic Hint Text

All user-facing instructions and hints that reference a Slack URL MUST use the configured `slackWorkspaceUrl` value instead of a hardcoded URL.

#### Scenario: Hint text reflects configured URL
- **GIVEN** `settings.json` contains `"slackWorkspaceUrl": "https://mycompany.slack.com"`
- **WHEN** `setup-browser` displays sign-in instructions
- **THEN** the instructions MUST reference `https://mycompany.slack.com` instead of `https://app.slack.com`

#### Scenario: Doctor hint text reflects configured URL
- **GIVEN** `settings.json` contains `"slackWorkspaceUrl": "https://mycompany.slack.com"`
- **WHEN** `doctor` reports no Slack tabs found
- **THEN** the hint MUST suggest opening `https://mycompany.slack.com`

## MODIFIED Requirements

None

## REMOVED Requirements

None
