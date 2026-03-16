## ADDED Requirements

### Requirement: Install as Background Service

The CLI MUST provide an `install` command that registers get-out as an hourly background service using the OS-native scheduling mechanism.

#### Scenario: Install on macOS
- **GIVEN** the user runs `get-out install` on macOS
- **WHEN** the command completes successfully
- **THEN** a launchd plist MUST exist at `~/Library/LaunchAgents/com.jflowers.get-out.plist` AND a wrapper script MUST exist at `~/.local/bin/get-out-wrapper.sh` AND the service MUST be loaded via `launchctl bootstrap`

#### Scenario: Install on Linux
- **GIVEN** the user runs `get-out install` on Linux
- **WHEN** the command completes successfully
- **THEN** a systemd service unit MUST exist at `~/.config/systemd/user/get-out.service` AND a timer unit MUST exist at `~/.config/systemd/user/get-out.timer` AND a wrapper script MUST exist at `~/.local/bin/get-out-wrapper.sh` AND the timer MUST be enabled via `systemctl --user enable --now`

#### Scenario: Service runs correct command
- **GIVEN** the background service is installed
- **WHEN** the service fires on schedule
- **THEN** it MUST execute `get-out export --sync --parallel 5` via the wrapper script

#### Scenario: Service runs hourly
- **GIVEN** the background service is installed
- **WHEN** 3600 seconds have elapsed since the last run (macOS) or the hourly calendar event fires (Linux)
- **THEN** the service MUST execute the wrapper script

### Requirement: Idempotent Install

Running `get-out install` multiple times MUST be safe and MUST result in a correctly configured service.

#### Scenario: Re-install overwrites cleanly
- **GIVEN** the service is already installed
- **WHEN** the user runs `get-out install` again
- **THEN** the wrapper script, plist/units MUST be overwritten AND the service MUST be reloaded without errors

### Requirement: Uninstall Service

The CLI MUST provide an `uninstall` command that stops and removes the background service.

#### Scenario: Uninstall on macOS
- **GIVEN** the service is installed on macOS
- **WHEN** the user runs `get-out uninstall`
- **THEN** the service MUST be unloaded via `launchctl bootout` AND the plist file MUST be removed AND the wrapper script MUST be removed

#### Scenario: Uninstall on Linux
- **GIVEN** the service is installed on Linux
- **WHEN** the user runs `get-out uninstall`
- **THEN** the timer MUST be disabled via `systemctl --user disable --now` AND the service and timer unit files MUST be removed AND the wrapper script MUST be removed AND `systemctl --user daemon-reload` MUST be run

#### Scenario: Uninstall when not installed
- **GIVEN** the service is not installed
- **WHEN** the user runs `get-out uninstall`
- **THEN** the command MUST complete without error (missing files are silently tolerated)

### Requirement: Config Directory Must Exist

The `install` command MUST verify that `~/.get-out/` exists before installing the service.

#### Scenario: Config directory missing
- **GIVEN** the config directory `~/.get-out/` does not exist
- **WHEN** the user runs `get-out install`
- **THEN** the command MUST fail with an error message telling the user to run `get-out init` first

### Requirement: Wrapper Script Features

The generated wrapper script MUST provide environment setup, log rotation, and exit code reporting.

#### Scenario: Environment file sourced
- **GIVEN** a file exists at `~/.get-out/.env`
- **WHEN** the wrapper script runs
- **THEN** the environment variables from `.env` MUST be exported before running the binary

#### Scenario: Log rotation
- **GIVEN** the log file exceeds 5 MB
- **WHEN** the wrapper script runs
- **THEN** the existing log file MUST be rotated to `.1` before the new run begins

#### Scenario: Timestamped logging
- **GIVEN** the wrapper script runs
- **WHEN** the export command starts and finishes
- **THEN** the wrapper MUST log the start time, end time, and exit code

### Requirement: Doctor Check for Service Status

The `doctor` command MUST include a check for whether the background sync service is installed.

#### Scenario: Service installed
- **GIVEN** the service plist (macOS) or timer unit (Linux) exists
- **WHEN** the user runs `get-out doctor`
- **THEN** the check MUST report pass with a message indicating the service is installed

#### Scenario: Service not installed
- **GIVEN** the service plist/timer does not exist
- **WHEN** the user runs `get-out doctor`
- **THEN** the check MUST report warn with a hint to run `get-out install`

## MODIFIED Requirements

_None._

## REMOVED Requirements

_None._
