## ADDED Requirements

### Requirement: Activity Indicator During Long Operations

The CLI MUST display an animated activity indicator during operations that may take more than 2 seconds, so the user can see the process is alive.

#### Scenario: Export shows spinner in default mode
- **GIVEN** the user runs `get-out export` without `--verbose` or `--debug` in an interactive terminal
- **WHEN** the export begins processing conversations
- **THEN** an animated spinner with a status message MUST be displayed on stderr, updating as the export progresses

#### Scenario: Export spinner updates with conversation progress
- **GIVEN** the export spinner is active during a multi-conversation export
- **WHEN** the exporter reports progress via the OnProgress callback
- **THEN** the spinner's status message MUST update to reflect the current operation (e.g., conversation name, message count)

#### Scenario: Spinner stops before summary output
- **GIVEN** the export spinner is active
- **WHEN** the export completes (successfully or with errors)
- **THEN** the spinner MUST stop and clear its line before the summary table is printed to stdout

### Requirement: Spinner Disabled in Verbose Mode

The spinner MUST NOT be displayed when `--verbose` or `--debug` flags are active, because verbose mode already provides line-by-line progress output.

#### Scenario: Verbose mode shows log lines instead of spinner
- **GIVEN** the user runs `get-out export --verbose`
- **WHEN** the exporter reports progress
- **THEN** progress MUST be printed as indented text lines (existing behavior) AND no spinner animation MUST be present

### Requirement: Spinner Disabled for Non-TTY Output

The spinner MUST NOT be displayed when stderr is not an interactive terminal (piped, redirected, or running in CI).

#### Scenario: Piped output has no spinner
- **GIVEN** the user runs `get-out export 2>/dev/null` or the process stderr is piped
- **WHEN** the export runs
- **THEN** no spinner animation MUST be written to stderr AND no ANSI escape sequences MUST be emitted

### Requirement: Discover Command Activity Indicator

The discover command MUST display a spinner during the member-fetching and user-profile-fetching phases.

#### Scenario: Discover shows spinner during user fetching
- **GIVEN** the user runs `get-out discover` in an interactive terminal
- **WHEN** the command is fetching conversation members and user profiles
- **THEN** a spinner with status updates MUST be displayed

### Requirement: Doctor Command Activity Indicator

The doctor command SHOULD display a brief spinner during slow checks (Chrome connection, Drive API validation).

#### Scenario: Doctor shows spinner during Chrome check
- **GIVEN** the user runs `get-out doctor` in an interactive terminal
- **WHEN** the Chrome connection check is in progress
- **THEN** a spinner MUST be displayed until the check completes, then replaced by the styled pass/fail result

## MODIFIED Requirements

_None._

## REMOVED Requirements

_None._
