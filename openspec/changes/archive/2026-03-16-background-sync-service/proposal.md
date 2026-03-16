## Why

Running `get-out export --sync --parallel 5` manually to keep Slack exports up to date is tedious and easy to forget. Users need a way to install get-out as a background service that runs the sync automatically on an hourly schedule, so exports stay current without any manual intervention.

The gcal-organizer project already has this pattern (`gcal-organizer install`/`uninstall`), and users expect the same convenience from get-out. The service should use the OS-native scheduling mechanism (launchd on macOS, systemd on Linux) for reliability, automatic restart after reboot, and integration with OS-level service management tools.

## What Changes

Add two new CLI commands: `get-out install` and `get-out uninstall`. The install command generates a wrapper script and an OS-native service definition that runs `get-out export --sync --parallel 5` every hour. The uninstall command stops the service and removes all generated files.

The service runs via a wrapper shell script (not the binary directly) to handle environment setup, log rotation, and exit code reporting. The binary path is resolved at install time via `os.Executable()` and embedded in the wrapper script, so the service always points to the correct binary regardless of how it was installed (Homebrew, go install, manual build).

## Capabilities

### New Capabilities
- `install`: CLI command that installs get-out as an hourly background service. On macOS, creates a launchd plist at `~/Library/LaunchAgents/com.jflowers.get-out.plist`. On Linux, creates systemd user units at `~/.config/systemd/user/get-out.service` and `get-out.timer`. Both platforms generate a wrapper script at `~/.local/bin/get-out-wrapper.sh`.
- `uninstall`: CLI command that stops and removes the background service, plist/units, and wrapper script.

### Modified Capabilities
- `doctor`: Adds a new check for service installation status — reports whether the hourly sync service is installed and provides a hint to run `get-out install` if not.

### Removed Capabilities
- None

## Impact

- **New file**: `internal/cli/service.go` (~300 lines) — install/uninstall commands, template generators for wrapper/plist/systemd units
- **Modified file**: `internal/cli/selfservice.go` — new doctor check for service status
- **Generated files at install time**: wrapper script, plist or systemd units (user's home directory, not in the repo)
- **No changes to `pkg/` packages**: The service feature is purely CLI-layer
- **No new Go dependencies**: Uses only `os/exec`, `os`, `fmt`, `path/filepath`, `runtime`, `text/template` from the standard library
- **Requires Chrome**: The service runs `export --sync` which uses browser mode for DM/group conversations. Chrome must be running with remote debugging enabled for the service to work. This is a pre-existing requirement, not a new constraint.

## Constitution Alignment

Assessed against the get-out project constitution.

### I. Session-Driven Extraction

**Assessment**: N/A

This change does not alter the extraction strategy. The service simply invokes the existing `export --sync --parallel 5` command on a schedule. Browser session handling, token extraction, and API mimicry behavior are unchanged.

### II. Go-First Architecture

**Assessment**: PASS

The install/uninstall commands are pure Go using only the standard library (`os/exec`, `text/template`, `os`). The generated wrapper script is bash, but it's a thin launcher — all business logic remains in the Go binary. No new external dependencies are introduced. The single-binary deployment model is preserved.

### III. Stealth & Reliability

**Assessment**: PASS

The service uses `--sync` mode, which exports only new messages since the last run. This produces consistent, low-volume API traffic at hourly intervals — stealthier than manual bulk exports. The systemd timer includes `RandomizedDelaySec=120` to avoid thundering-herd patterns. The launchd plist uses `RunAtLoad=true` with `StartInterval=3600` for reliable hourly scheduling.

### IV. Testability

**Assessment**: PASS

The template generation functions (`generateWrapper`, `generatePlist`, `generateSystemdService`, `generateSystemdTimer`) are pure functions that return strings — testable in isolation without any OS service infrastructure. The install/uninstall commands that interact with `launchctl`/`systemctl` are thin wrappers around `exec.Command` and `os.WriteFile`, tested via the existing CLI test patterns.
