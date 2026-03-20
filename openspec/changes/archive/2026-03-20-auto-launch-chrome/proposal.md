## Why

The `setup-browser` command currently fails at Step 1 if Chrome isn't already running with `--remote-debugging-port=9222`. It only prints a hint command for the user to copy-paste and run manually. This creates unnecessary friction — especially for first-time users who must context-switch to a terminal, run the Chrome command, then re-run `setup-browser`.

The `gcal-organizer` project (a sibling tool in this ecosystem) solves this well: its `setup-browser` command automatically launches Chrome with a dedicated profile, polls until the debugging port is ready, and interactively guides the user through authentication. get-out should adopt the same pattern.

## What Changes

Transform `setup-browser` from a passive verification tool into an active setup wizard that:

1. Creates and manages a dedicated Chrome profile directory (`~/.get-out/chrome-data/`)
2. Automatically launches Chrome with `--remote-debugging-port` and `--user-data-dir` when it's not already running
3. Opens `https://app.slack.com` as the initial URL so the user lands directly on the Slack login page
4. Presents an interactive "Press Enter to verify" prompt after Chrome launches, giving the user time to authenticate
5. Then proceeds with the existing Slack tab detection, credential extraction, and API validation steps

## Capabilities

### New Capabilities
- `chrome-profile-management`: Automatically creates and reuses a dedicated Chrome profile at `~/.get-out/chrome-data/`, isolating get-out's browser state from the user's personal Chrome
- `chrome-auto-launch`: Detects Chrome binary on macOS and Linux, launches it with remote debugging and the dedicated profile when port is not already active
- `port-readiness-polling`: Polls the debugging port up to 20 times at 500ms intervals (10s total) after launching Chrome
- `interactive-auth-prompt`: Displays a styled box prompt with first-run vs returning-user messaging, waits for Enter key before proceeding to verification
- `auto-navigate-slack`: Opens `https://app.slack.com` as Chrome's initial URL so the user lands directly on Slack

### Modified Capabilities
- `setup-browser`: Restructured from 5 passive check steps to a 5-step active wizard (profile setup → Chrome launch → auth prompt → credential extraction → API validation)

### Removed Capabilities
- None. The `chromeLaunchCmd()` hint function is retained for the `doctor` command.

## Impact

- **`internal/cli/selfservice.go`**: Major changes — new helper functions (`chromeProfilePath`, `findChromeBinary`, `launchChrome`, `isPortOpen`), rewritten `runSetupBrowser` flow
- **`internal/cli/selfservice_test.go`**: New tests for helper functions and updated wizard flow
- **`pkg/chrome/`**: No changes required — existing `Connect()`, `ListTargets()`, `FindSlackTarget()`, `ExtractCredentials()` work as-is
- **Other commands**: `doctor`, `export`, `discover` are unaffected — they continue to assume Chrome is already running
- **New dependency**: `os/exec` (stdlib only, no external deps)
- **New filesystem artifact**: `~/.get-out/chrome-data/` directory created on first run

## Constitution Alignment

Assessed against the get-out project constitution.

### I. Session-Driven Extraction

**Assessment**: PASS

This change strengthens session-driven extraction by actively managing the browser session lifecycle. Instead of requiring users to manually launch Chrome with the correct flags, get-out now handles it directly — reducing the chance of misconfiguration and making session-based authentication more accessible.

### II. Go-First Architecture

**Assessment**: PASS

All new functionality is implemented in Go using only stdlib packages (`os/exec`, `net`, `bufio`). No external dependencies are added. The single-binary deployment model is preserved.

### III. Stealth & Reliability

**Assessment**: PASS

The dedicated Chrome profile uses `--user-data-dir` which creates a headed browser instance with normal profile state — consistent with the constitution's requirement to "use headed profile state" and avoid headless detection. The auto-launch approach mirrors how gcal-organizer handles this reliably.

### IV. Two-Tier Extraction Strategy

**Assessment**: N/A

This change does not modify the extraction strategy. It only affects how Chrome is launched and connected to before extraction begins.

### V. Concurrency & Resilience

**Assessment**: PASS

Port readiness polling (20 retries × 500ms) ensures resilient startup. Non-interactive terminal detection gracefully skips the interactive prompt. Chrome already running on the port is detected and reused without launching a second instance.

### VI. Security First

**Assessment**: PASS

No credentials are stored or persisted by this change. The dedicated Chrome profile stores browser state (cookies, localStorage) in `~/.get-out/chrome-data/` which inherits the OS-level file permissions of the user's home directory. This follows the same pattern approved for `token.json` and `credentials.json`.

### VII. Output Format

**Assessment**: N/A

No changes to export output format.

### VIII. Google Drive Integration

**Assessment**: N/A

No changes to Google Drive integration.

### IX. Documentation Maintenance

**Assessment**: PASS

The tasks include updating README.md and AGENTS.md to reflect the new `setup-browser` behavior and the `~/.get-out/chrome-data/` directory.
