# Tasks: Distribution, Config Migration & Self-Service Commands

**Input**: Design documents from `/specs/003-distribution/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/cli-schema.md, quickstart.md

**Organization**: Tasks grouped by user story (P1→P5) to enable independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no shared state dependencies)
- **[Story]**: Which user story this task belongs to (US1–US5)
- All paths are relative to repository root

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Add new dependencies, create new files, remove deleted files — unblocks all subsequent work.

- [ ] T001 Add `charmbracelet/huh` and `charmbracelet/lipgloss` dependencies: `go get github.com/charmbracelet/huh@latest github.com/charmbracelet/lipgloss@latest && go mod tidy`
- [ ] T002 [P] Create `internal/cli/helpers.go` — move `safePreview()` from `internal/cli/test.go` here (package `cli`); verify `internal/cli/export_test.go` still compiles
- [ ] T003 [P] Create `internal/cli/selfservice.go` — empty file with `package cli` and import stubs for cobra, huh, lipgloss; register no commands yet
- [ ] T004 Delete `internal/cli/test.go` (after T002 confirms `safePreview` moved); run `go build ./...` to confirm no compilation errors
- [ ] T005 [P] Add `FolderID string \`json:"folder_id,omitempty"\`` field to `Settings` struct in `pkg/config/types.go`
- [ ] T006 [P] Add exported `DefaultConfigDir() string` function to `pkg/config/config.go` — returns `~/.get-out/` resolved from `os.UserHomeDir()`
- [ ] T007 Change `defaultConfigDir()` in `internal/cli/root.go` to call `config.DefaultConfigDir()` instead of returning `~/.config/get-out`; run `go build ./...`
- [ ] T008 Create `man/` directory and stub `man/get-out.1` with the troff NAME section only — full content added in Polish phase

**Checkpoint**: `go build ./...` and `go test -race -count=1 ./...` pass; `get-out --help` works; default config dir is `~/.get-out/`.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Shared lipgloss style constants and `export` folder-ID default — used by all subsequent commands.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [ ] T009 Add shared lipgloss style constants to `internal/cli/selfservice.go`: `passStyle` (color 10, green), `warnStyle` (color 11, yellow), `failStyle` (color 9, red); add `pass(msg)`, `warn(msg)`, `fail(msg)` helper functions that write to stdout
- [ ] T010 In `internal/cli/export.go`: after loading `settings`, if the `--folder-id` flag is empty and `settings.FolderID != ""`, set the effective folder ID to `settings.FolderID`; add `strings` import if needed; run `go build ./...`

**Checkpoint**: `go build ./...` passes; `go test -race -count=1 ./...` passes.

---

## Phase 3: User Story 1 — First-Time Setup via Homebrew (Priority: P1) 🎯 MVP

**Goal**: Deliver `get-out init` with config dir creation, migration detection, folder-ID prompt, and Next Steps box; plus the release pipeline (signing, Homebrew cask) so the binary is installable via `brew install get-out`.

**Independent Test**: A fresh macOS machine with Homebrew can run `brew tap jflowers/tools && brew install get-out && get-out init` and end up with `~/.get-out/` scaffolded, folder ID saved to `settings.json`, and a "Next Steps" box printed — without reading any source code.

### Implementation for User Story 1

- [ ] T011 [US1] Implement `runInit` in `internal/cli/selfservice.go`:
  - Step 1: `os.MkdirAll(configDir, 0700)`; if `~/.get-out/` already existed as a file, detect and exit with error
  - Step 2: Migration — if `~/.config/get-out/` exists, iterate managed files (`settings.json`, `conversations.json`, `people.json`, `credentials.json`, `token.json`); for each file present in old dir but absent in new dir, copy it; use `os.OpenFile(..., 0600)` for `credentials.json`/`token.json`, standard `os.WriteFile` for others; print notice per file copied
  - Step 3: Write `conversations.json` template `{"conversations":[]}` if absent
  - Step 4: Write `settings.json` template `{}` if absent
  - Step 5: Folder-ID prompt (see T012); skip if `--non-interactive` or `settings.FolderID != ""`
  - Step 6: Print Next Steps box
  - Register `initCmd` in `selfservice.go`'s `init()`, add to `rootCmd`
  - Add `--non-interactive` bool flag

- [ ] T012 [US1] Implement folder-ID prompt using `charmbracelet/huh` in `internal/cli/selfservice.go`:
  - `huh.NewInput()` with title "Google Drive Folder ID", description pointing to Drive URL
  - Validate function: `len(id) >= 28 && regexp.MustCompile("^[a-zA-Z0-9_-]+$").MatchString(id)`
  - On accept: load/parse `settings.json`, set `FolderID`, marshal back, write to `settings.json`
  - Skip prompt entirely when stdin is not a TTY (piped input — check with `os.Stdin.Stat()`)

- [ ] T013 [P] [US1] Create `jflowers/homebrew-tools` GitHub repository:
  - `gh repo create jflowers/homebrew-tools --public --description "Homebrew tap for jflowers tools"`
  - Create `README.md`: "Install: `brew tap jflowers/tools && brew install <tool>`"
  - Create `Casks/get-out.rb` skeleton (placeholder SHA, version `:latest`)
  - Push initial commit

- [ ] T014 [P] [US1] Add GitHub Actions secrets to `jflowers/get-out` repo:
  - `gh secret set MACOS_SIGN_P12 --repo jflowers/get-out` (value from env.md)
  - `gh secret set MACOS_SIGN_PASSWORD --repo jflowers/get-out`
  - `gh secret set MACOS_NOTARY_KEY --repo jflowers/get-out`
  - `gh secret set MACOS_NOTARY_KEY_ID --repo jflowers/get-out` (value: `4K669B7BD9`)
  - `gh secret set MACOS_NOTARY_ISSUER_ID --repo jflowers/get-out` (value: `f3feda93-660b-47a6-a402-7f95d678ca7c`)
  - `gh secret set HOMEBREW_TAP_TOKEN --repo jflowers/get-out` (same PAT as gcal-organizer)

- [ ] T015 [P] [US1] Update `.goreleaser.yml`:
  - Add `checksum: name_template: checksums.txt` block
  - Add `man/get-out.1` to `archives[0].files` list
  - Add `homebrew_casks` block pointing to `jflowers/homebrew-tools`, `Casks/`, token `{{ .Env.HOMEBREW_TAP_TOKEN }}`, binary `get-out`, manpage `man/get-out.1`, `skip_upload: auto`

- [ ] T016 [US1] Rewrite `.github/workflows/release.yml` to add `sign-macos` job adapted from gcal-organizer:
  - `release` job: add `outputs: has_signing_secrets` (probe `MACOS_SIGN_P12`); add `HOMEBREW_TAP_TOKEN: ${{ secrets.HOMEBREW_TAP_TOKEN }}` env to GoReleaser step
  - `sign-macos` job: `needs: release`, `if: needs.release.outputs.has_signing_secrets == 'true'`, `runs-on: macos-latest`, `timeout-minutes: 30`
  - Steps: import `.p12` into temp keychain → decode `.p8` → download `get-out_*_darwin_*.tar.gz` → for each arch: extract binary, `codesign --force --timestamp --options runtime --sign "Developer ID Application: John Flowers (PGFWLVZX55)"`, verify, zip, `xcrun notarytool submit --wait --timeout 20m`, re-archive (binary + LICENSE + `man/get-out.1`) → upload with `--clobber` → update `checksums.txt` → clone `jflowers/homebrew-tools`, patch `Casks/get-out.rb` SHA256 with awk, commit+push

- [ ] T017 [US1] Write unit tests for `init` helper functions in `internal/cli/selfservice_test.go`:
  - Test `validateDriveID`: valid IDs (28+ alphanumeric), invalid IDs (too short, invalid chars)
  - Test migration copy logic with temp dirs: file absent in new dir gets copied; file present in new dir is not overwritten; `credentials.json` copied with mode 0600

**Checkpoint**: `get-out init` runs on a clean machine, creates `~/.get-out/`, prompts for folder ID, prints Next Steps. `go test -race -count=1 ./...` passes. A manual tag push produces a signed macOS release.

---

## Phase 4: User Story 2 — Existing User Config Migration (Priority: P2)

**Goal**: Ensure `get-out init` correctly detects and migrates files from `~/.config/get-out/` on upgrade. `get-out doctor` warns when old dir still exists.

**Independent Test**: A machine with `~/.config/get-out/` populated (conversations.json, credentials.json, token.json) and no `~/.get-out/` runs `get-out init`; all absent files are copied to `~/.get-out/`; `get-out list` reads from the new location.

### Implementation for User Story 2

- [ ] T018 [US2] Verify migration logic in `runInit` (implemented in T011) handles all edge cases:
  - `~/.config/get-out/` absent → no migration attempt, no error
  - `~/.get-out/` non-empty + old dir exists → only copy files absent from new dir (verify existing files untouched)
  - Corrupt file in old dir → copy as-is; let `doctor` catch it
  - Add test cases to `internal/cli/selfservice_test.go` for each edge case using temp directories

- [ ] T019 [US2] In `internal/cli/selfservice.go` `runDoctor`, add old-dir warning check: after all 10 checks, if `~/.config/get-out/` exists AND `~/.get-out/` exists, call `warn(...)` with message suggesting `rm -rf ~/.config/get-out/`; increment `warnCount`

**Checkpoint**: Running `get-out init` on a machine with the old dir migrates files correctly. Running `get-out doctor` with both dirs present shows the warning. `go test -race -count=1 ./...` passes.

---

## Phase 5: User Story 3 — Health Check Before Export (Priority: P3)

**Goal**: `get-out doctor` runs 10 checks with styled pass/warn/fail output, prints a summary line, exits 0 on warn-only, exits 1 on any failure.

**Independent Test**: A machine with a missing `token.json` and Chrome not running runs `get-out doctor`; checks 3 and 8 show ✗ with corrective actions; summary shows failures; process exits with code 1.

### Implementation for User Story 3

- [ ] T020 [US3] Implement `runDoctor` skeleton in `internal/cli/selfservice.go`:
  - Declare `passCount`, `warnCount`, `failCount int`
  - Define `checkResult` struct: `name`, `status` (pass/warn/fail), `message`, `fix`, `path`
  - Print header separator line
  - After all checks: print summary `── N passed · M warnings · P failures ──`
  - `if failCount > 0 { os.Exit(1) }` (not cobra `RunE` error return, to preserve styled output)
  - Register `doctorCmd` in `selfservice.go`'s `init()`, add to `rootCmd`

- [ ] T021 [P] [US3] Implement checks 1–3 in `runDoctor` in `internal/cli/selfservice.go`:
  - Check 1 (config dir): `os.Stat(configDir)` — fail if absent; warn if perms != 0700 (suggest `chmod 700`)
  - Check 2 (credentials.json): `os.Stat(filepath.Join(configDir, "credentials.json"))` — fail if absent with Google Console URL
  - Check 3 (token.json): `os.Stat(filepath.Join(configDir, "token.json"))` — fail if absent with `get-out auth login`
  - File permission check for credentials.json / token.json: warn if mode broader than 0600

- [ ] T022 [P] [US3] Implement checks 4–5 in `runDoctor` in `internal/cli/selfservice.go`:
  - Check 4 (token valid): load token via `gdrive.LoadToken(cfg)`; if expired + refresh token present → warn (auto-refresh capable); if expired + no refresh → fail
  - Check 5 (Drive API): create gdrive client, call `drive.About.Get`; fail with error message if unsuccessful; skip if check 4 failed

- [ ] T023 [P] [US3] Implement checks 6–7 in `runDoctor` in `internal/cli/selfservice.go`:
  - Check 6 (conversations.json): `config.LoadConversations(path)` — fail if missing/invalid JSON; warn if 0 conversations
  - Check 7 (people.json): `os.Stat(...)` — warn (not fail) if absent; suggest `get-out discover`

- [ ] T024 [P] [US3] Implement checks 8–10 in `runDoctor` in `internal/cli/selfservice.go`:
  - Check 8 (Chrome port): HTTP GET `http://127.0.0.1:<chromePort>/json/version` with 2s timeout; fail if no response; print OS-appropriate Chrome launch command
  - Check 9 (Slack tab): call `session.ListTargets()`, filter with `chrome.IsSlackURL`; warn if 0 tabs (not fail); skip if check 8 failed
  - Check 10 (export-index.json): `exporter.LoadExportIndex(path)` — warn if absent (first run); fail if present but corrupt JSON

- [ ] T025 [US3] Wire `--verbose` flag to `runDoctor`: when `verbose` global flag is true, append resolved file path to each check row output in `internal/cli/selfservice.go`

**Checkpoint**: `get-out doctor` with a fully configured environment prints 10 green ✓ rows and "10/10 passed", exits 0. With a missing token it prints ✗ on check 3 and exits 1. `go test -race -count=1 ./...` passes.

---

## Phase 6: User Story 4 — Authentication Status and Re-auth (Priority: P3)

**Goal**: `get-out auth` becomes a command group; `auth login` and `auth status` are sub-commands. `auth status` is read-only and exits non-zero when auth is broken.

**Independent Test**: A user with a valid token runs `get-out auth status` and sees credentials, token, expiry, and email — no browser opens. A user with no `token.json` sees "Not authenticated" and process exits 1.

### Implementation for User Story 4

- [ ] T026 [US4] Refactor `internal/cli/auth.go`:
  - Remove `RunE` from `authCmd`; change `Short` to "Manage Google authentication"; `Use` stays "auth"
  - Move existing login logic into new `authLoginCmd` (`Use: "login"`, `RunE: runAuthLogin`); register as child of `authCmd`
  - Update `init()` to register `authLoginCmd` and `authStatusCmd` as children of `authCmd`

- [ ] T027 [US4] Implement `authStatusCmd` (`Use: "status"`, `RunE: runAuthStatus`) in `internal/cli/auth.go`:
  - Check 1: `os.Stat(credentialsPath)` — print "Credentials: ✓ found" or "✗ not found"
  - Check 2: `os.Stat(tokenPath)` — print "Token: ✓ found" or "✗ not found"; if not found, print "→ Run: get-out auth login" and `return fmt.Errorf("not authenticated")`
  - Check 3: load token; call `gdrive.EnsureTokenFresh(ctx, cfg)` (silent refresh if possible; saves refreshed token); check `token.Valid()`; print expiry
  - Check 4: if token valid, call `drive.About.Get`; print connected email
  - Exit non-zero (return error from `RunE`) if token absent, expired with no refresh, or Drive call fails

**Checkpoint**: `get-out auth login` works as before. `get-out auth status` prints 4-row status table without opening a browser. `get-out auth` alone prints help. `go test -race -count=1 ./...` passes.

---

## Phase 7: User Story 5 — Browser Setup Wizard (Priority: P4)

**Goal**: `get-out setup-browser` replaces `get-out test` with a guided 5-step wizard using lipgloss styling. Steps skip on failure. OS-appropriate Chrome launch instructions on step 1 failure.

**Independent Test**: Chrome not running → step 1 fails with macOS launch command → steps 2–5 reported as Skipped → process exits 1. Chrome running with Slack tab → all 5 steps pass → process exits 0.

### Implementation for User Story 5

- [ ] T028 [US5] Implement `runSetupBrowser` in `internal/cli/selfservice.go`:
  - Declare `stepFailed bool` sentinel
  - Step 1: HTTP GET `http://127.0.0.1:<chromePort>/json/version` with 3s timeout; on failure set `stepFailed = true`; print OS-appropriate launch command (`runtime.GOOS == "darwin"` → `open -a "Google Chrome" --args --remote-debugging-port=<port>`; else → `google-chrome --remote-debugging-port=<port>`)
  - Step 2: if `stepFailed` → print Skipped; else `chrome.NewSession(ctx, chromePort).ListTargets()`, print tab count
  - Step 3: if `stepFailed` → print Skipped; else filter targets with `chrome.IsSlackURL`; if 0 tabs → `warn(...)` but do NOT set `stepFailed`; if target found → print URL
  - Step 4: if `stepFailed` → print Skipped; else `chrome.ExtractCredentials(ctx, session)`; on failure set `stepFailed = true`; on success print `safePreview(token)` (from `helpers.go`)
  - Step 5: if `stepFailed` → print Skipped; else `slackapi.NewBrowserClient(token, cookie, "").ValidateAuth(ctx)`; print workspace + username on success
  - Print footer: "Setup complete. Run: get-out export" if all passed; else instructions
  - `if stepFailed { os.Exit(1) }`
  - Register `setupBrowserCmd` in `selfservice.go`'s `init()`, add to `rootCmd`

**Checkpoint**: `get-out setup-browser` with Chrome not running shows step 1 fail + 4 skips, exits 1. With Chrome + Slack tab shows all 5 pass, exits 0. `go test -race -count=1 ./...` passes.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Man page, documentation updates, final validation.

- [ ] T029 Write `man/get-out.1` in troff format with sections: NAME, SYNOPSIS, DESCRIPTION, COMMANDS (one `.TP` per command: init, auth login, auth status, doctor, setup-browser, discover, list, export, status), OPTIONS (global flags), FILES (`~/.get-out/` contents table), EXAMPLES, AUTHOR
- [ ] T030 [P] Update `README.md`:
  - Add Homebrew install section at top: `brew tap jflowers/tools && brew install get-out`
  - Update Quick Start: `init` → `auth login` → `setup-browser` → `export`
  - Add `doctor` to troubleshooting section
  - Remove references to `test` command; replace with `setup-browser`
- [ ] T031 [P] Update `AGENTS.md`:
  - Default config dir → `~/.get-out/`
  - Command list: add `init`, `doctor`, `auth login`, `auth status`, `setup-browser`; remove `test`
  - Add `charmbracelet/huh`, `charmbracelet/lipgloss` to Active Technologies
  - Update test command if needed
- [ ] T032 [P] Update `internal/cli/root.go` `Long` help text: replace Quick Start example `get-out auth` with `get-out init` + `get-out auth login`; remove `test` reference
- [ ] T033 Run `go build ./...` and `go test -race -count=1 ./...` — all must pass
- [ ] T034 Manual validation against `specs/003-distribution/quickstart.md`:
  - Run `get-out init` on a clean `~/.get-out/` (or temp dir via `--config`)
  - Run `get-out auth status` — verify output format
  - Run `get-out doctor` — verify all checks run and summary correct
  - Run `get-out setup-browser` with Chrome running + Slack tab
  - Verify `get-out auth login` still works
- [ ] T035 Commit all changes and push branch `003-distribution`; open PR to `main`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 completion — BLOCKS all user stories
- **Phase 3 (US1 — init + release pipeline)**: Depends on Phase 2; T013/T014/T015/T016 can run in parallel with T011/T012/T017
- **Phase 4 (US2 — migration)**: Depends on Phase 2; shares `runInit` with US1 — start after T011 completes
- **Phase 5 (US3 — doctor)**: Depends on Phase 2; fully independent of US1/US2
- **Phase 6 (US4 — auth sub-commands)**: Depends on Phase 2; fully independent of US1/US2/US3
- **Phase 7 (US5 — setup-browser)**: Depends on Phase 2; fully independent
- **Phase 8 (Polish)**: Depends on all user story phases complete

### User Story Dependencies

- **US1 (P1)**: After Phase 2 — no story dependencies
- **US2 (P2)**: After T011 completes (shares `runInit`) — minimal dependency
- **US3 (P3)**: After Phase 2 — no story dependencies; T021–T024 parallelizable within phase
- **US4 (P3)**: After Phase 2 — no story dependencies
- **US5 (P4)**: After Phase 2 — no story dependencies; builds on `chrome` and `slackapi` packages already in place

### Parallel Opportunities

- T002, T003, T005, T006 can all run in parallel (different files)
- T007 depends on T006; T004 depends on T002
- T013, T014, T015, T016 (release pipeline) can all run in parallel with each other and with T011/T012
- T021, T022, T023, T024 (doctor checks 1–10) can run in parallel (different check groups, same file — coordinate via sequential merge)
- T029, T030, T031, T032 can all run in parallel (different files)

---

## Parallel Execution Example: Phase 3 (US1)

```
# These can start in parallel after Phase 2:
Task T011: Implement runInit (internal/cli/selfservice.go)
Task T013: Create homebrew-tools repo (GitHub)
Task T014: Set GitHub Actions secrets (GitHub)
Task T015: Update .goreleaser.yml

# T016 depends on T015 (GoReleaser config must exist first):
Task T016: Rewrite release.yml with sign-macos job

# T012 depends on T011 (prompt is called from runInit):
Task T012: Implement huh folder-ID prompt

# T017 depends on T011+T012:
Task T017: Write unit tests for init helpers
```

---

## Implementation Strategy

### MVP First (User Story 1 + Release Pipeline)

1. Complete Phase 1: Setup (T001–T008)
2. Complete Phase 2: Foundational (T009–T010)
3. Complete Phase 3: US1 — `init` command + signing + Homebrew cask (T011–T017)
4. **STOP and VALIDATE**: Run `get-out init --config /tmp/test-config`; manually push a test tag and verify signed release + cask update
5. Ship if the Homebrew install and signed binary work end-to-end

### Incremental Delivery

1. Setup + Foundational → Foundation ready
2. US1 (`init` + release pipeline) → Homebrew users can install and initialize
3. US2 (migration) → Existing users upgrade cleanly
4. US3 (`doctor`) → Self-service health check live
5. US4 (`auth login/status`) → Auth sub-commands restructured
6. US5 (`setup-browser`) → Guided Chrome wizard replaces `test`
7. Polish (man page, docs) → Complete

### Single Developer Strategy

Sequential in priority order is recommended since US1 (`init`) and the release pipeline are the highest-value deliverable. US3–US5 can each be completed in a single session.

---

## Project Structure Reference

```text
internal/cli/
├── root.go              # EDIT T007: defaultConfigDir → config.DefaultConfigDir()
├── auth.go              # REFACTOR T026+T027: auth group + login + status
├── export.go            # EDIT T010: folder_id default from settings
├── selfservice.go       # NEW T003/T009/T011/T020+: init, doctor, setup-browser
├── helpers.go           # NEW T002: safePreview, shared formatting
├── export_test.go       # UNCHANGED (safePreview now in helpers.go, same package)
└── selfservice_test.go  # NEW T017/T018: unit tests for init/migration helpers

pkg/config/
├── config.go            # EDIT T006: add DefaultConfigDir()
└── types.go             # EDIT T005: add FolderID to Settings

man/
└── get-out.1            # NEW T008 (stub) → T029 (full content)

.goreleaser.yml          # EDIT T015
.github/workflows/
└── release.yml          # REWRITE T016
README.md                # EDIT T030
AGENTS.md                # EDIT T031
```

---

## Notes

- `[P]` tasks touch different files or independent GitHub resources — safe to parallelise
- `selfservice.go` is written by multiple tasks; coordinate sequentially within the same session to avoid conflicts
- `doctor` exit-code behaviour uses `os.Exit(1)` directly (not `RunE` error return) to preserve styled summary output before exit
- Migration never overwrites existing files in `~/.get-out/` — this is the safety invariant; tests in T017/T018 must assert this explicitly
- The `sign-macos` job patches only darwin SHA256 values in the cask; linux values are set correctly by GoReleaser's initial cask commit and must not be overwritten
