## 1. Create StatusSpinner Component

- [x] 1.1 Create `internal/cli/spinner.go` with the `StatusSpinner` struct: fields for `mu sync.Mutex`, `message string`, `writer io.Writer`, `frames []string`, `interval time.Duration`, `done chan struct{}`, `active bool`, `style lipgloss.Style`
- [x] 1.2 Implement `NewStatusSpinner() *StatusSpinner` — initialize with `spinner.MiniDot` frames, 100ms interval, stderr as writer, dim lipgloss style for the spinner character
- [x] 1.3 Implement `Start()` — launch a goroutine that ticks at `interval`, writes `\r` + styled frame + message to writer on each tick; set `active = true`
- [x] 1.4 Implement `Update(msg string)` — set message under the mutex; the next tick picks it up
- [x] 1.5 Implement `Stop()` — signal the goroutine to exit via `done` channel, clear the spinner line with `\r` + spaces + `\r`, set `active = false`
- [x] 1.6 Extract `isTerminal()` from `selfservice.go` into a shared location (e.g., `helpers.go` or `spinner.go`) so both spinner and selfservice can use it

## 2. Test StatusSpinner

- [x] 2.1 Test `NewStatusSpinner` — returns non-nil spinner with expected defaults (frames, interval, writer)
- [x] 2.2 Test `Start/Stop` lifecycle — start writes frames to a `bytes.Buffer` writer, stop clears the line; verify no goroutine leak
- [x] 2.3 Test `Update` — calling Update changes the message that appears in subsequent writes
- [x] 2.4 Test `Stop` before `Start` — calling Stop on an unstarted spinner does not panic
- [x] 2.5 Test concurrent `Update` calls — multiple goroutines calling Update simultaneously do not race (verify with `-race`)

## 3. Wire Spinner into Export Command

- [x] 3.1 In `runExport`, after "Starting export...", create and start a `StatusSpinner` if `!verbose && !debugMode && isTerminal()`
- [x] 3.2 Modify the `OnProgress` callback: when spinner is active, call `spin.Update(msg)` instead of `fmt.Printf`; when verbose/debug, keep existing `fmt.Printf` behavior
- [x] 3.3 Stop the spinner before printing the summary table (both success and error paths)
- [x] 3.4 Stop the spinner in the interrupt signal handler before "Saving progress..." message

## 4. Wire Spinner into Discover Command

- [x] 4.1 In `runDiscover`, start a spinner before the member-fetching loop if `isTerminal()`
- [x] 4.2 Replace inline `fmt.Printf` calls for per-conversation member fetching with spinner updates
- [x] 4.3 Update the spinner during the user-profile-fetching phase (e.g., "Fetching user profiles... 25/100")
- [x] 4.4 Stop the spinner before printing the final "Wrote N people to..." message

## 5. Wire Spinner into Doctor and Setup-Browser

- [x] 5.1 In `runDoctor`, start a brief spinner before `checkDriveAPI()` with message "Checking Drive API access..."; stop it and print the styled result after the check returns
- [x] 5.2 In `runDoctor`, start a brief spinner before `checkSlackTab()` with message "Checking Slack tab..."; stop it and print the styled result
- [x] 5.3 In `runSetupBrowser`, add a spinner during each wizard step's slow operation (Chrome connect, credential extraction, auth validation); stop before printing the step result

## 6. Verification

- [x] 6.1 Run full test suite: `go test -race -count=1 ./...` — all tests MUST pass
- [x] 6.2 Run gaze CRAPload check: verify CRAPload <= 10 is maintained
- [x] 6.3 Manual test: run `get-out export --dry-run` — verify spinner does NOT appear (dry-run is fast, no export phase)
- [x] 6.4 Manual test: run `get-out doctor` — verify spinner appears during slow checks and clears before styled results
- [x] 6.5 Verify no spinner when stderr is piped: `get-out export 2>/dev/null` should produce no ANSI escape sequences on stderr
- [x] 6.6 Verify verbose mode: `get-out export -v` should show indented log lines with no spinner
- [x] 6.7 Verify constitution alignment: no new external dependencies, spinner testable in isolation, no changes to pkg/ packages
