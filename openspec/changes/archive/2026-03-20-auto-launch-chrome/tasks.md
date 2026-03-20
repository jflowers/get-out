## 1. Helper Functions

- [x] 1.1 Add `chromeProfilePath() (string, error)` to `internal/cli/selfservice.go` — returns `filepath.Join(home, ".get-out", "chrome-data")` using `os.UserHomeDir()`
- [x] 1.2 Add `findChromeBinary() string` to `internal/cli/selfservice.go` — macOS: return `/Applications/Google Chrome.app/Contents/MacOS/Google Chrome` if it exists; Linux: search PATH for `google-chrome`, `google-chrome-stable`, `chromium-browser` via `exec.LookPath`; return `""` if not found
- [x] 1.3 Add `isPortOpen(port int) bool` to `internal/cli/selfservice.go` — TCP dial to `127.0.0.1:<port>` with 500ms timeout, return true if connection succeeds
- [x] 1.4 Add `launchChrome(profilePath string, port int) (*exec.Cmd, error)` to `internal/cli/selfservice.go` — call `findChromeBinary()`, build `exec.Cmd` with args `--remote-debugging-port=<port>`, `--user-data-dir=<profilePath>`, `https://app.slack.com`; set stdout/stderr to nil; call `cmd.Start()`; return error if binary not found or start fails
- [x] 1.5 Add new imports to `internal/cli/selfservice.go`: `bufio`, `net`, `os/exec` (keep existing imports)

## 2. Rewrite `runSetupBrowser`

- [x] 2.1 Replace Step 1 (Chrome reachability check) with Chrome profile step: call `chromeProfilePath()`, check if dir exists with `os.Stat()`, create with `os.MkdirAll()` if not, set `firstRun` flag, display pass/fail with profile path
- [x] 2.2 Replace Step 2 (list tabs) with Chrome launch step: call `isPortOpen(chromePort)` — if open, display "Chrome is already running on port N"; if not, call `launchChrome(profilePath, chromePort)`, then poll `isPortOpen()` up to 20 times at 500ms intervals, display result
- [x] 2.3 Add new Step 3 — interactive Slack authentication prompt: check `isTerminal()`; if non-interactive, display warning and skip; if interactive, render boxStyle prompt with first-run or returning-user messaging; wait for `bufio.NewReader(os.Stdin).ReadString('\n')`
- [x] 2.4 Renumber existing Step 3 (find Slack tab + extract credentials) as Step 4: connect via CDP, list targets, find Slack tab, extract credentials — consolidate the old steps 3 and 4 into one step since they are logically coupled
- [x] 2.5 Renumber existing Step 5 (validate against Slack API) as Step 5: unchanged logic, just renumbered
- [x] 2.6 Update the header banner and footer to match the new step count and messaging

## 3. Tests

- [x] 3.1 Add `TestChromeProfilePath` to `internal/cli/selfservice_test.go` — verify returned path ends with `.get-out/chrome-data` and is under the home directory
- [x] 3.2 Add `TestFindChromeBinary` to `internal/cli/selfservice_test.go` — verify returns non-empty on the current test platform (macOS CI has Chrome); skip on platforms without Chrome
- [x] 3.3 Add `TestIsPortOpen` to `internal/cli/selfservice_test.go` — start a `net.Listen("tcp", ":0")` on a random port, verify `isPortOpen` returns true; verify returns false for an unused port
- [x] 3.4 Add `TestLaunchChrome_NoBinary` to `internal/cli/selfservice_test.go` — verify `launchChrome()` returns error when `findChromeBinary()` returns empty (may require test helper or build tag to simulate missing binary)

## 4. Documentation

- [x] 4.1 Update `README.md` — revise the `setup-browser` section to describe the new auto-launch behavior, dedicated profile, and interactive prompt
- [x] 4.2 Update `AGENTS.md` — add `~/.get-out/chrome-data/` to the project structure section; update the `setup-browser` command description

## 5. Verification

- [x] 5.1 Run `go build -o get-out ./cmd/get-out` — verify successful compilation
- [x] 5.2 Run `go test -race -count=1 ./...` — verify all tests pass including new ones
- [ ] 5.3 Manual smoke test: run `get-out setup-browser` with Chrome not running — verify Chrome auto-launches with Slack URL
- [ ] 5.4 Manual smoke test: run `get-out setup-browser` with Chrome already running on port 9222 — verify it skips launch and proceeds to verification
- [x] 5.5 Verify constitution alignment: no external deps added (Go-First); dedicated profile is isolated (Composability); credentials not persisted by this change (Security First); new helpers are testable in isolation (Testability)
