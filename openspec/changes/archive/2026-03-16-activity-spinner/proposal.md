## Why

The export command outputs nothing between "Starting export..." and the final summary table. For a multi-conversation export that takes minutes or hours, the terminal appears completely frozen. Users have no way to know whether the process is active, stuck, or crashed without passing `--verbose`.

The discover command fares better with inline per-conversation messages, but they're plain unstyled text. The doctor and setup-browser commands already use lipgloss-styled output but have no animation during slow checks (Chrome connection, Drive API validation).

All three commands would benefit from an animated spinner with a status message that tells the user what's happening right now.

## What Changes

Add a `StatusSpinner` component in `internal/cli/` that wraps the `bubbles/spinner` frame definitions with a lightweight inline animation loop. The spinner writes to stderr via carriage return (`\r`) to overwrite a single terminal line — no full Bubble Tea program takeover.

The spinner is wired into three commands:
- **export**: Shows the active conversation name, message count, and progress (e.g., "Exporting general (1/5)... 150 messages fetched")
- **discover**: Shows member/user fetching progress
- **doctor/setup-browser**: Shows "Checking..." during the slow Chrome and Drive API checks

The spinner is disabled automatically when `--verbose` is active (verbose mode already provides status updates via `fmt.Printf`) or when stderr is not a TTY (piped/redirected output).

## Capabilities

### New Capabilities
- `StatusSpinner`: A lightweight inline spinner component in `internal/cli/spinner.go` with `Start()`, `Update(msg)`, and `Stop()` methods. Uses `bubbles/spinner` frame definitions (MiniDot style) and lipgloss for styling.

### Modified Capabilities
- `export`: In default mode (no `--verbose`), displays an animated spinner with conversation name and progress instead of silence. Verbose mode is unchanged.
- `discover`: Displays spinner during member-fetching and user-profile-fetching phases instead of plain `fmt.Printf` lines.
- `doctor`: Displays brief spinner during Chrome connection check and Drive API check before printing the styled pass/fail result.
- `setup-browser`: Displays brief spinner during each wizard step's slow operation.

### Removed Capabilities
- None

## Impact

- **User experience**: The most significant UX improvement — users can see at a glance that the process is alive and what it's doing. No more "is it frozen?" uncertainty during long exports.
- **Files changed**: `internal/cli/spinner.go` (new), `internal/cli/export.go`, `internal/cli/discover.go`, `internal/cli/selfservice.go`
- **No changes to `pkg/` packages**: The spinner is purely a CLI presentation concern. The exporter's `OnProgress` callback contract is unchanged.
- **No new dependencies**: `bubbles/spinner` is already in `go.mod` as a transitive dependency of `huh`. This change promotes it to a direct import but adds no new modules.
- **Backward compatibility**: Output format is unchanged when `--verbose` is active or when stderr is not a TTY. Only default interactive mode gains the spinner.

## Constitution Alignment

Assessed against the get-out project constitution.

### I. Session-Driven Extraction

**Assessment**: N/A

This change does not alter the extraction strategy. No changes to Chrome CDP, Slack API calls, or authentication behavior. The spinner is a presentation-layer addition only.

### II. Go-First Architecture

**Assessment**: PASS

The spinner uses `bubbles/spinner` which is already in the Go module graph as a transitive dependency. No external tools, no new languages, no new binaries. The single-binary deployment model is preserved.

### III. Stealth & Reliability

**Assessment**: N/A

The spinner does not affect network behavior, rate limiting, or API interactions. It only changes what the user sees in their terminal during operations.

### IV. Testability

**Assessment**: PASS

The `StatusSpinner` is a standalone type with no external dependencies beyond the terminal. Its core logic (frame cycling, message formatting, TTY detection) can be tested in isolation. The export/discover commands' `OnProgress` callbacks remain the same — the spinner is wired in at the CLI layer, not inside the library packages.
