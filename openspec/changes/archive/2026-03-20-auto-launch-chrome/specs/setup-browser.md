## ADDED Requirements

### Requirement: Chrome Profile Management

The `setup-browser` command MUST create and manage a dedicated Chrome profile directory at `~/.get-out/chrome-data/`. The directory MUST be created if it does not exist. The command MUST detect whether this is a first-run (directory newly created) or a returning run (directory already exists) and adjust user messaging accordingly.

#### Scenario: First run — profile directory does not exist
- **GIVEN** the directory `~/.get-out/chrome-data/` does not exist
- **WHEN** the user runs `get-out setup-browser`
- **THEN** the directory `~/.get-out/chrome-data/` MUST be created
- **AND** `firstRun` MUST be set to `true` for downstream messaging

#### Scenario: Returning run — profile directory exists
- **GIVEN** the directory `~/.get-out/chrome-data/` already exists
- **WHEN** the user runs `get-out setup-browser`
- **THEN** the existing directory MUST be reused without modification
- **AND** `firstRun` MUST be set to `false`

---

### Requirement: Chrome Binary Detection

The command MUST locate the Chrome binary on the host system. On macOS, it MUST check `/Applications/Google Chrome.app/Contents/MacOS/Google Chrome`. On Linux, it MUST search `PATH` for `google-chrome`, `google-chrome-stable`, and `chromium-browser` (in that order) using `exec.LookPath`. The function MUST return an empty string if no binary is found.

#### Scenario: Chrome found on macOS
- **GIVEN** the OS is macOS (`runtime.GOOS == "darwin"`)
- **AND** `/Applications/Google Chrome.app/Contents/MacOS/Google Chrome` exists
- **WHEN** `findChromeBinary()` is called
- **THEN** it MUST return `/Applications/Google Chrome.app/Contents/MacOS/Google Chrome`

#### Scenario: Chrome found on Linux via PATH
- **GIVEN** the OS is Linux (`runtime.GOOS == "linux"`)
- **AND** `google-chrome-stable` is in `PATH` but `google-chrome` is not
- **WHEN** `findChromeBinary()` is called
- **THEN** it MUST return the full path resolved by `exec.LookPath` for `google-chrome-stable`

#### Scenario: Chrome not found
- **GIVEN** no Chrome binary is found at any known location
- **WHEN** `findChromeBinary()` is called
- **THEN** it MUST return an empty string

---

### Requirement: Chrome Auto-Launch

When Chrome is not already running on the debugging port, the command MUST launch Chrome automatically with `--remote-debugging-port=<port>`, `--user-data-dir=<profilePath>`, and `https://app.slack.com` as the initial URL. Chrome MUST be launched as a background process using `cmd.Start()`. Chrome's stdout and stderr MUST be suppressed (set to `nil`).

#### Scenario: Chrome not running — auto-launch succeeds
- **GIVEN** port 9222 is not in use
- **AND** Chrome binary is found on the system
- **WHEN** the user runs `get-out setup-browser`
- **THEN** Chrome MUST be launched with `--remote-debugging-port=9222 --user-data-dir=~/.get-out/chrome-data/ https://app.slack.com`
- **AND** the command MUST poll the port for readiness

#### Scenario: Chrome not running — binary not found
- **GIVEN** port 9222 is not in use
- **AND** no Chrome binary is found
- **WHEN** the user runs `get-out setup-browser`
- **THEN** the command MUST display an error: "Chrome not found. Install Google Chrome and try again"
- **AND** the command MUST exit with a non-zero status

#### Scenario: Chrome already running on port
- **GIVEN** port 9222 is already in use
- **WHEN** the user runs `get-out setup-browser`
- **THEN** no new Chrome process MUST be launched
- **AND** the command MUST display "Chrome is already running on port 9222"

---

### Requirement: Port Readiness Polling

After launching Chrome, the command MUST poll the debugging port for readiness. It MUST attempt up to 20 TCP connection attempts at 500ms intervals (10 seconds total). Each attempt MUST use `net.DialTimeout` with a 500ms timeout. If the port becomes ready, the command MUST proceed. If the port is not ready after all attempts, a warning SHOULD be displayed but the command SHOULD NOT fail immediately.

#### Scenario: Port becomes ready within polling window
- **GIVEN** Chrome was just launched
- **WHEN** the debugging port becomes available after 2 seconds
- **THEN** `isPortOpen()` MUST return `true` on the 4th or 5th poll attempt
- **AND** the command MUST display a success message

#### Scenario: Port not ready after polling exhausted
- **GIVEN** Chrome was just launched
- **WHEN** the debugging port does not become available within 10 seconds
- **THEN** the command MUST display a warning: "Chrome started but port not yet ready"
- **AND** the command MUST continue to the interactive prompt (not abort)

---

### Requirement: Interactive Authentication Prompt

After Chrome is launched (or confirmed running), the command MUST display an interactive prompt using a styled box (lipgloss `boxStyle`). The prompt content MUST vary based on whether this is a first-run or returning-run. The command MUST wait for the user to press Enter before proceeding to credential extraction.

If stdin is not a terminal (detected via `isTerminal()`), the prompt MUST be skipped with a warning message.

#### Scenario: First-run interactive prompt
- **GIVEN** this is a first run (`firstRun == true`)
- **AND** stdin is a terminal
- **WHEN** Chrome has been launched successfully
- **THEN** the command MUST display a box with instructions to sign in to Slack
- **AND** the command MUST wait for the user to press Enter

#### Scenario: Returning-user interactive prompt
- **GIVEN** this is a returning run (`firstRun == false`)
- **AND** stdin is a terminal
- **WHEN** Chrome is running
- **THEN** the command MUST display a box with "You should already be signed in. Press Enter to verify..."
- **AND** the command MUST wait for the user to press Enter

#### Scenario: Non-interactive environment
- **GIVEN** stdin is not a terminal (e.g., piped input, CI environment)
- **WHEN** the command reaches the interactive prompt step
- **THEN** the prompt MUST be skipped
- **AND** a warning MUST be displayed: "Non-interactive terminal — skipping authentication prompt"

---

### Requirement: Port Flag Consistency

The `--chrome-port` global flag (default 9222) MUST be used for both the Chrome launch argument (`--remote-debugging-port=<port>`) and all subsequent port checks and CDP connections. The port MUST NOT be hardcoded within the setup-browser implementation.

#### Scenario: Custom port flag
- **GIVEN** the user runs `get-out setup-browser --chrome-port 9333`
- **WHEN** Chrome is auto-launched
- **THEN** Chrome MUST be launched with `--remote-debugging-port=9333`
- **AND** port readiness polling MUST check port 9333
- **AND** CDP connection MUST use port 9333

## MODIFIED Requirements

### Requirement: setup-browser Step Structure

The `setup-browser` command step structure MUST change from 5 passive verification steps to 5 active wizard steps.

Previously: Steps were (1) check Chrome reachable, (2) list tabs, (3) find Slack tab, (4) extract credentials, (5) validate API. Step 1 failure caused all subsequent steps to be skipped with "Skipped" output.

New step structure:
1. **Chrome profile** — Check/create dedicated profile directory
2. **Launch Chrome** — Check port, auto-launch if needed, poll readiness
3. **Slack authentication** — Interactive prompt for user to sign in
4. **Verify Slack & extract credentials** — Connect via CDP, find Slack tab, extract token+cookie
5. **Validate against Slack API** — Call `ValidateAuth()`, display team/user

Steps 4-5 remain functionally identical to the previous steps 3-5 (find Slack tab, extract, validate) but are renumbered.

#### Scenario: Full successful flow
- **GIVEN** Chrome is not running and the profile does not exist
- **WHEN** the user runs `get-out setup-browser`
- **THEN** all 5 steps MUST execute in sequence
- **AND** the final output MUST display "Setup complete. Run: get-out export"

## REMOVED Requirements

None. All existing functionality is preserved or enhanced. The `chromeLaunchCmd()` hint function remains available for the `doctor` command.
