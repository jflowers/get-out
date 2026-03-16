## 1. Create Service File and Template Generators

- [x] 1.1 Create `internal/cli/service.go` with package declaration, imports, and cobra command vars `installCmd` and `uninstallCmd` registered to `rootCmd`
- [x] 1.2 Implement `generateWrapper(binaryPath, logPath, envFile string) string` — returns the bash wrapper script content with environment sourcing, log rotation at 5 MB, timestamped logging, and the `get-out export --sync --parallel 5` invocation
- [x] 1.3 Implement `generatePlist(wrapperPath, logPath, home, binaryPath string) string` — returns the macOS launchd plist XML with Label `com.jflowers.get-out`, StartInterval 3600, RunAtLoad true, PATH including `/opt/homebrew/bin`
- [x] 1.4 Implement `generateSystemdService(wrapperPath, home string) string` — returns the systemd service unit with Type=oneshot, ExecStart=/bin/bash wrapperPath
- [x] 1.5 Implement `generateSystemdTimer() string` — returns the systemd timer unit with OnCalendar=hourly, Persistent=true, RandomizedDelaySec=120

## 2. Implement Install Command

- [x] 2.1 Implement `runInstall` — resolve binary path via `os.Executable()`, check `~/.get-out/` exists (fail with hint if not), dispatch to `installMacOS` or `installLinux` based on `runtime.GOOS`
- [x] 2.2 Implement `installMacOS(binaryPath string) error` — create `~/.local/bin/` dir, write wrapper script (0755), write plist to `~/Library/LaunchAgents/`, run `launchctl bootout` (ignore errors), run `launchctl bootstrap gui/<uid>`, print success summary with status/log/remove hints
- [x] 2.3 Implement `installLinux(binaryPath string) error` — create `~/.local/bin/` dir, write wrapper script (0755), create `~/.config/systemd/user/` dir, write service and timer units, run `systemctl --user daemon-reload`, run `systemctl --user enable --now get-out.timer`, print success summary

## 3. Implement Uninstall Command

- [x] 3.1 Implement `runUninstall` — dispatch to `uninstallMacOS` or `uninstallLinux` based on `runtime.GOOS`
- [x] 3.2 Implement `uninstallMacOS() error` — run `launchctl bootout` (ignore errors), remove plist, remove wrapper script, print confirmation; tolerate missing files
- [x] 3.3 Implement `uninstallLinux() error` — run `systemctl --user disable --now get-out.timer` (ignore errors), remove service unit, remove timer unit, remove wrapper script, run `systemctl --user daemon-reload`, print confirmation; tolerate missing files

## 4. Add Doctor Check

- [x] 4.1 Add `isServiceInstalled() bool` helper — check for plist (macOS) or timer unit (Linux) existence
- [x] 4.2 Add service installation check to `runDoctor` — report pass if installed, warn with hint to run `get-out install` if not

## 5. Test Template Generators

- [x] 5.1 Test `generateWrapper` — verify output contains the binary path, log rotation logic, environment sourcing, and the export command
- [x] 5.2 Test `generatePlist` — verify output is valid XML-like structure containing the label, interval, wrapper path, and log path
- [x] 5.3 Test `generateSystemdService` — verify output contains ExecStart with wrapper path and Type=oneshot
- [x] 5.4 Test `generateSystemdTimer` — verify output contains OnCalendar=hourly and Persistent=true
- [x] 5.5 Test `isServiceInstalled` — verify returns false when no plist/timer exists (uses temp dir)

## 6. Verification

- [x] 6.1 Run full test suite: `go test -race -count=1 ./...` — all tests MUST pass
- [x] 6.2 Run gaze CRAPload check: verify CRAPload <= 10 is maintained
- [x] 6.3 Manual test: run `get-out install` on macOS — verify plist and wrapper are created, service is loaded
- [x] 6.4 Manual test: run `get-out uninstall` — verify plist and wrapper are removed, service is unloaded
- [x] 6.5 Manual test: run `get-out doctor` — verify service check reports pass/warn correctly
- [x] 6.6 Verify no new Go module dependencies: `go.mod` unchanged
- [x] 6.7 Verify constitution alignment: template generators testable in isolation, no changes to pkg/ packages, no new external dependencies
