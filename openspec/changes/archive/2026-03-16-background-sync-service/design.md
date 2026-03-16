## Context

Users currently run `get-out export --sync --parallel 5` manually to keep their Slack exports up to date. The gcal-organizer project provides `install`/`uninstall` commands that register the tool as an hourly OS-native background service. get-out needs the same pattern, adapted for its specific command and configuration directory.

The existing CLI commands live in `internal/cli/` as cobra commands registered to `rootCmd`. The self-service commands (`init`, `doctor`, `setup-browser`) are in `selfservice.go`. The install/uninstall commands follow the same structure.

## Goals / Non-Goals

### Goals
- Install get-out as an hourly background service using OS-native scheduling (launchd on macOS, systemd on Linux)
- Run `get-out export --sync --parallel 5` on each invocation
- Generate a wrapper script that handles environment setup, log rotation, and exit code reporting
- Make install idempotent — running it multiple times safely overwrites previous installations
- Make uninstall clean — removes all generated files and stops the service
- Add a doctor check for service installation status

### Non-Goals
- Windows support (not a target platform for this project)
- Configurable command — the service always runs `export --sync --parallel 5`
- Configurable interval via flags — hardcoded to hourly (can be changed by editing the plist/unit directly)
- Daemon mode (long-running process with internal scheduler) — we use the OS scheduler
- Automatic Chrome startup — Chrome must already be running with remote debugging for browser-mode export

## Decisions

### 1. New file: internal/cli/service.go

All install/uninstall logic lives in a single new file, following the pattern of `selfservice.go` for the existing self-service commands. This keeps the service concerns isolated and the file discoverable.

### 2. Wrapper script pattern (not direct binary invocation)

The service runs a bash wrapper script, not the Go binary directly. This provides:

- **Environment setup**: Sources `~/.get-out/.env` if it exists, for any user-defined environment variables
- **Log rotation**: Rotates the log file at 5 MB, keeping one `.1` backup (max ~10 MB disk usage)
- **Timestamped logging**: Adds start/end timestamps and exit code to each run
- **Shell safety**: The binary path is single-quoted to handle paths with spaces or metacharacters

The wrapper script is generated at `~/.local/bin/get-out-wrapper.sh` with mode 0755.

### 3. Binary path resolution via os.Executable()

The binary path is resolved at install time using `os.Executable()`, which returns the absolute path of the currently running binary. This is embedded into the wrapper script, so the service always invokes the correct binary regardless of how it was installed:

- `brew install get-out` → `/opt/homebrew/bin/get-out`
- `go install ./cmd/get-out` → `~/go/bin/get-out` or `~/go/1.25.0/bin/get-out`
- Manual build → wherever the user built it

If the user upgrades the binary (e.g., `brew upgrade`), they should re-run `get-out install` to update the wrapper path.

### 4. macOS: launchd LaunchAgent

**Plist location**: `~/Library/LaunchAgents/com.jflowers.get-out.plist`
**Label**: `com.jflowers.get-out`

Key plist properties:
- `StartInterval: 3600` — runs every hour
- `RunAtLoad: true` — runs immediately when loaded (after login/reboot)
- `ProgramArguments`: `/bin/bash <wrapperPath>`
- `StandardOutPath` and `StandardErrorPath`: `~/Library/Logs/get-out.log`
- `EnvironmentVariables.PATH`: includes `/opt/homebrew/bin` for Apple Silicon Macs

**Install**: `launchctl bootstrap gui/<uid> <plistPath>`
**Uninstall**: `launchctl bootout gui/<uid> <plistPath>`

Both `bootout` and `bootstrap` are used because the older `load`/`unload` commands are deprecated on modern macOS.

### 5. Linux: systemd user units

**Service**: `~/.config/systemd/user/get-out.service`
- `Type=oneshot` — runs and exits (not a long-running daemon)
- `ExecStart=/bin/bash <wrapperPath>`

**Timer**: `~/.config/systemd/user/get-out.timer`
- `OnCalendar=hourly`
- `Persistent=true` — catches up missed runs after sleep/reboot
- `RandomizedDelaySec=120` — up to 2 minutes of jitter

**Install**: `systemctl --user daemon-reload && systemctl --user enable --now get-out.timer`
**Uninstall**: `systemctl --user disable --now get-out.timer`, remove files, `daemon-reload`

### 6. Idempotent install

Running `get-out install` multiple times is safe:
- macOS: `launchctl bootout` (error ignored) before `launchctl bootstrap`
- Linux: `systemctl daemon-reload` before `enable --now`
- Wrapper script, plist, and units are overwritten via `os.WriteFile`

### 7. Config directory check

If `~/.get-out/` does not exist, the install command prints an error telling the user to run `get-out init` first, rather than auto-running init (which requires interactive input for the folder ID prompt).

### 8. Doctor check for service status

A new doctor check inspects whether the service is installed:
- macOS: checks if `~/Library/LaunchAgents/com.jflowers.get-out.plist` exists
- Linux: checks if `~/.config/systemd/user/get-out.timer` exists

Reports pass (installed) or warn (not installed, with hint to run `get-out install`).

### 9. User output on install

The install command prints a summary of what was created and how to interact with the service:

```
Installing get-out as an hourly background service...
  Binary: /opt/homebrew/bin/get-out
  Wrapper: ~/.local/bin/get-out-wrapper.sh
  Service: ~/Library/LaunchAgents/com.jflowers.get-out.plist
  Interval: hourly
  Log: ~/Library/Logs/get-out.log
  Command: get-out export --sync --parallel 5

  ✓ Service installed and started

To check status: launchctl list | grep get-out
To view logs: tail -f ~/Library/Logs/get-out.log
To remove: get-out uninstall
```

### 10. Template generation as pure functions

All template generators (`generateWrapper`, `generatePlist`, `generateSystemdService`, `generateSystemdTimer`) are pure functions that accept parameters and return strings. This satisfies the Testability principle — they can be tested without any OS service infrastructure.

## Risks / Trade-offs

### Chrome must be running for browser-mode exports

The service runs `export --sync` which uses Chrome CDP for DM/group conversations. If Chrome is not running with remote debugging enabled, those conversations will fail. The service will still export API-mode conversations (channels with bot token) successfully.

**Mitigation**: The doctor command already checks Chrome reachability. The service logs failures, so the user can diagnose via the log file.

### Binary path becomes stale after upgrades

If the user upgrades the binary (e.g., `brew upgrade get-out`), the wrapper script still points to the old path. The old path may no longer exist (Homebrew removes old versions).

**Mitigation**: Document that users should re-run `get-out install` after upgrading. The install command could also detect a stale path and warn, but this is deferred.

### Log file grows without bound on Linux

On macOS, the wrapper script handles log rotation (5 MB max). On Linux, systemd's journald handles logging, so the wrapper's log rotation code is still present but the systemd output goes to the journal instead. The wrapper's own logging (start/end timestamps) goes to the journal too.

**Mitigation**: The wrapper script's log rotation only applies when launchd redirects stdout/stderr to a file. On Linux, the journal handles log retention via journald configuration.
