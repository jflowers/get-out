## Context

The `setup-browser` command currently operates as a passive diagnostic: it checks whether Chrome is running on port 9222 and, if not, prints a shell command for the user to run manually. This creates a two-step workflow (run hint command → re-run setup-browser) that is especially painful for first-time users.

The sibling project `gcal-organizer` has a proven pattern: launch Chrome programmatically with a dedicated profile, poll for readiness, and interactively guide the user through authentication. This design adapts that pattern for get-out's Slack-focused workflow.

All changes are confined to `internal/cli/selfservice.go` (CLI layer). The `pkg/chrome/` package requires no modifications — its `Connect()`, `ListTargets()`, and `ExtractCredentials()` APIs already support the new flow.

## Goals / Non-Goals

### Goals
- Automatically launch Chrome with remote debugging when it's not already running
- Use a dedicated Chrome profile (`~/.get-out/chrome-data/`) to isolate get-out's browser state
- Open `https://app.slack.com` as the initial URL so the user lands on Slack immediately
- Provide an interactive pause (Press Enter) so the user can authenticate before verification proceeds
- Detect first-run vs returning-user scenarios and adjust messaging accordingly
- Support both macOS and Linux Chrome binary locations
- Remain idempotent: if Chrome is already running on the port, skip launch

### Non-Goals
- Modifying `pkg/chrome/` — the CDP client layer is stable and sufficient
- Supporting Windows — the project targets macOS and Linux (consistent with existing `chromeLaunchCmd()`)
- Managing Chrome process lifecycle beyond launch — no shutdown/cleanup hooks
- Adding Chrome profile path as a config file option — the path is conventional (`~/.get-out/chrome-data/`)
- Changing `doctor`, `export`, or `discover` commands — they continue to assume Chrome is already running

## Decisions

### D1: Chrome profile location at `~/.get-out/chrome-data/`

The profile lives inside the existing `~/.get-out/` config directory. This keeps all get-out state in one place and follows the same convention as `gcal-organizer` (`~/.gcal-organizer/chrome-data/`). The directory is created with default permissions (inheriting the user's umask), consistent with how `~/.get-out/` itself is created by the `init` command.

Constitution alignment: Composability First — the profile is self-contained within get-out's directory; no shared state with other tools.

### D2: Chrome binary detection via `findChromeBinary()`

On macOS, the binary is at the well-known path `/Applications/Google Chrome.app/Contents/MacOS/Google Chrome`. On Linux, we search `PATH` for `google-chrome`, `google-chrome-stable`, and `chromium-browser` using `exec.LookPath`. This mirrors the gcal-organizer approach.

The function returns an empty string if Chrome is not found, letting the caller produce a clear error message rather than a cryptic exec failure.

### D3: Background process launch with `cmd.Start()`

Chrome is launched with `cmd.Start()` (not `cmd.Run()`), so it runs as a background child process. `cmd.Stdout` and `cmd.Stderr` are set to `nil` to suppress Chrome's noisy stderr output (GPU warnings, etc.). The get-out CLI does not manage the Chrome process after launch — when the CLI exits, Chrome continues running independently (it's not killed on parent exit because `exec.Cmd` doesn't set `SysProcAttr.Setpgid`).

### D4: Port readiness polling (20 × 500ms = 10s)

After launching Chrome, poll `isPortOpen()` up to 20 times at 500ms intervals. This 10-second window matches the gcal-organizer's timing and is sufficient for Chrome startup on typical hardware. If the port isn't ready after 10s, a warning is shown (not a fatal error) and the wizard continues — the interactive prompt gives the user additional time.

### D5: `isPortOpen()` uses raw TCP dial, not HTTP

Port checking uses `net.DialTimeout("tcp", ...)` with a 500ms timeout, not an HTTP request to `/json/version`. The TCP check is faster and sufficient for determining if Chrome's debugging server is listening. The full HTTP/CDP connection happens later when `chrome.Connect()` is called for credential extraction.

### D6: Interactive prompt via `bufio.NewReader(os.Stdin)`

The "Press Enter to verify" prompt uses `bufio.NewReader(os.Stdin).ReadString('\n')` rather than `charmbracelet/huh`. Rationale:
- It's a trivial single-key interaction that doesn't need a TUI framework
- Matches the gcal-organizer implementation
- Gracefully handles non-interactive terminals (stdin closed/piped) by catching the error and skipping the pause

The `isTerminal()` helper (already in `helpers.go`) is used to detect non-interactive environments and skip the prompt entirely with a warning.

### D7: First-run detection based on profile directory existence

If `~/.get-out/chrome-data/` doesn't exist, the user is on their first run. This triggers:
- More detailed instructions ("Sign in to your Slack workspace...")
- Creation of the profile directory before Chrome launch

Returning users see a shorter message ("You should already be signed in. Press Enter to verify...").

### D8: Respect `--chrome-port` flag for launch args

The Chrome launch command uses the `chromePort` variable (from the global `--chrome-port` flag, default 9222) in its `--remote-debugging-port` argument. This ensures consistency: if a user runs `get-out setup-browser --chrome-port 9333`, Chrome launches on port 9333 and subsequent verification checks that same port.

### D9: Rewritten step structure

The current 5 steps (check → list → find → extract → validate) become:

| Step | Name | Action |
|------|------|--------|
| 1 | Chrome profile | Check/create `~/.get-out/chrome-data/` |
| 2 | Launch Chrome | Check port → launch if needed → poll readiness |
| 3 | Slack authentication | Interactive prompt (first-run vs returning messaging) |
| 4 | Verify Slack & extract credentials | Connect via CDP, find Slack tab, extract token+cookie |
| 5 | Validate against Slack API | Call `slackClient.ValidateAuth()`, display team/user |

Steps 4-5 remain functionally identical to the current steps 3-5 but are renumbered.

## Risks / Trade-offs

### R1: Chrome already open without debugging

If the user has Chrome running normally (without `--remote-debugging-port`), launching a second Chrome instance with a different `--user-data-dir` will open a separate window. This is safe — Chrome supports multiple simultaneous profiles. However, it may confuse users who expect their existing Chrome window to be used. The wizard output should make it clear that a *new* dedicated Chrome window was opened.

### R2: Port collision

If another process (not Chrome) is listening on port 9222, `isPortOpen()` returns true and the wizard skips Chrome launch. The subsequent `chrome.Connect()` call in Step 4 will fail with a connection error, which surfaces clearly. This is an acceptable trade-off — the error is diagnostic.

### R3: Chrome binary not found

On non-standard installations (e.g., Chrome installed in a custom location), `findChromeBinary()` returns empty and the wizard reports a clear error. The fallback behavior is the same as today: the user sees a message to install Chrome.

### R4: Child process management

The launched Chrome process is not tracked or cleaned up by get-out. If the user runs `setup-browser` multiple times, each invocation checks the port first and skips launch if Chrome is already running. The only scenario that creates orphan processes is if Chrome crashes between the port check and the launch — extremely unlikely and harmless since the user can close Chrome windows manually.

### R5: No `--user-data-dir` on already-running Chrome

If Chrome is already running on the port (launched manually or by a previous `setup-browser` run), we cannot know or control which profile it's using. The wizard proceeds with whatever Chrome instance is available. This is intentional — the dedicated profile is a convenience for auto-launch, not a hard requirement.
