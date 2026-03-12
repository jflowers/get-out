# Research: Distribution, Config Migration & Self-Service Commands

**Feature**: 003-distribution  
**Date**: 2026-03-11  
**Status**: Complete — all NEEDS CLARIFICATION resolved

---

## 1. Config Directory Convention (`~/.get-out/`)

**Decision**: Use `~/.get-out/` as the canonical config directory (no XDG, no `$APPDATA`).

**Rationale**: The project targets a single developer persona on macOS/Linux. XDG (`~/.config/get-out/`) is correct by spec but creates friction for users who browse their home directory looking for tool config. The `~/.toolname/` pattern is well established for developer CLIs (e.g., `~/.aws/`, `~/.kube/`, `~/.docker/`, `~/.ssh/`). gcal-organizer already uses `~/.gcal-organizer/`; consistency across the jflowers toolchain reduces cognitive overhead.

**Alternatives considered**:
- `~/.config/get-out/` (XDG) — rejected: was the old default; migration is needed regardless; XDG is not well-known outside Linux power users
- `$APPDATA/get-out` (Windows) — rejected: project does not target Windows; CGO_ENABLED=0 + darwin/linux targets only
- Per-project config (current dir) — rejected: tool is user-global, not project-scoped

**Migration approach**: Copy-on-first-run in `init`. Files absent from `~/.get-out/` are copied from `~/.config/get-out/` if the old dir exists. Existing files in `~/.get-out/` are never overwritten. Old dir is never deleted automatically.

---

## 2. File Permission Model

**Decision**: Config dir `0700`; `credentials.json` and `token.json` written with `os.WriteFile(..., 0600)`; all other files use process umask default (typically `0644`).

**Rationale**: `0600` for credential files is the UNIX convention (same as `~/.ssh/id_rsa`, `~/.netrc`, `~/.pgpass`). The `0700` directory provides defense-in-depth at the directory level. Other config files (`conversations.json`, `people.json`, `settings.json`) contain no secrets and do not require per-file hardening.

**`doctor` check**: Check #1 verifies the directory exists at `0700`. An additional check (FR-023a) warns if `credentials.json` or `token.json` have permissions broader than `0600`.

**Implementation note**: Use `os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)` rather than `os.WriteFile` for the two credential files to ensure mode is set atomically at creation (umask is not applied to `os.OpenFile` with explicit mode when the file is being created fresh; on Linux/macOS `os.WriteFile` uses `0666` before umask).

---

## 3. `charmbracelet/huh` — Interactive Prompts in `init`

**Decision**: Use `charmbracelet/huh` v0.6+ for the folder-ID prompt in `get-out init`.

**Rationale**: gcal-organizer already uses huh for its `init` wizard; using the same library ensures visual consistency across tools and avoids adding a second TUI dependency. huh is a pure Go library with no CGO requirements.

**Usage pattern**:
```go
var folderID string
err := huh.NewForm(
    huh.NewGroup(
        huh.NewInput().
            Title("Google Drive Folder ID").
            Description("Found in the folder URL: drive.google.com/drive/folders/<ID>").
            Validate(validateDriveID).
            Value(&folderID),
    ),
).Run()
```

**Validate function**: `validateDriveID` checks `len(id) >= 28 && regexp.MustCompile(`^[a-zA-Z0-9_-]+$`).MatchString(id)`. This is a heuristic — Google does not publish a formal Drive ID grammar, but all observed IDs are 28–44 alphanumeric/hyphen/underscore characters.

**Non-interactive fallback**: When `--non-interactive` is set or stdin is not a TTY (piped), the `huh` form is skipped entirely; `folder_id` is left unset in `settings.json`.

---

## 4. `charmbracelet/lipgloss` — Styled Output in `doctor` and `setup-browser`

**Decision**: Use `charmbracelet/lipgloss` v1.x for pass/warn/fail row styling in `doctor` and `setup-browser`.

**Rationale**: Same library used in gcal-organizer; pure Go; no CGO; renders gracefully on non-colour terminals (lipgloss detects no-color environments and disables ANSI codes).

**Colour palette** (ANSI 256 — same as gcal-organizer for consistency):
- Pass: `lipgloss.Color("10")` — bright green
- Warn: `lipgloss.Color("11")` — bright yellow  
- Fail: `lipgloss.Color("9")` — bright red

**Symbol set**: `✓` (pass), `⚠` (warn), `✗` (fail) — render correctly on macOS Terminal, iTerm2, and VS Code terminal. Falls back gracefully on terminals that don't support Unicode (the lipgloss style still applies, only the symbol may render as `?`).

---

## 5. Apple Code Signing and Notarization Pipeline

**Decision**: Adapt gcal-organizer's `sign-macos` GitHub Actions job directly, with binary name and archive pattern substituted for `get-out`.

**Rationale**: The gcal-organizer pipeline is battle-tested. It handles the full chain: `.p12` import into temp keychain → `codesign` with `--timestamp --options runtime` → `ditto` zip for `notarytool` → `xcrun notarytool submit --wait` → re-archive → upload with `--clobber` → checksum update → tap cask patch. Reusing it minimises risk.

**Key parameters**:
- Signing identity: `Developer ID Application: John Flowers (PGFWLVZX55)`
- Notarization key ID: `4K669B7BD9`
- Notarization issuer: `f3feda93-660b-47a6-a402-7f95d678ca7c`
- Team ID: `PGFWLVZX55`

**Graceful degradation**: The `release` job probes for `MACOS_SIGN_P12` in its secrets and outputs `has_signing_secrets=true/false`. The `sign-macos` job runs only when `has_signing_secrets == 'true'`. Unsigned releases still publish all four platform binaries; only the Homebrew cask SHA update is skipped.

**No stapling**: Apple does not support stapling notarization tickets to bare Mach-O binaries (only `.app` bundles and disk images). Gatekeeper verifies via the Apple Notarization CDN on first run (requires network on first launch only).

**Alternatives considered**:
- `rcodesign` (Rust-based cross-platform signing) — rejected: adds complexity; the CI runner is already `macos-latest` which has `codesign` and `xcrun` natively
- GitHub-hosted macOS runners with persistent keychain — rejected: temp keychain per job is the secure pattern (keychain destroyed after job)

---

## 6. Homebrew Cask vs. Formula

**Decision**: Homebrew **cask** (not formula) in tap `jflowers/homebrew-tools`.

**Rationale**: A cask allows platform-specific URLs and SHA256 values (`on_macos { on_intel { ... } on_arm { ... } }`, `on_linux { ... }`), which is required because signed darwin archives have different checksums than unsigned linux archives. A formula (`install` method) cannot cleanly express per-arch checksums for prebuilt binaries. GoReleaser's `homebrew_casks` stanza auto-generates the correct multi-arch cask structure.

**Tap name**: `jflowers/tools` (Homebrew strips the `homebrew-` prefix from the repo name `homebrew-tools`). Install command: `brew tap jflowers/tools && brew install get-out`.

**Cask contents**: binary (`get-out`) + man page (`man/get-out.1`). No `pkg` or `app` bundle. No `zap` stanza (user's `~/.get-out/` directory should not be deleted on uninstall).

**Post-signing SHA patch**: The `sign-macos` job patches the cask's `sha256` values for darwin arches after uploading signed archives, using the same `awk`-based approach as gcal-organizer. Linux SHA values are set correctly by GoReleaser's initial cask commit and do not need patching.

---

## 7. `auth` Sub-Command Restructure

**Decision**: `authCmd` becomes a group (no `RunE`); `authLoginCmd` and `authStatusCmd` are registered as children.

**Rationale**: Matches cobra best practices for command groups (`Use: "auth"`, `Short: "Manage Google authentication"`, no `RunE` on the parent). The bare `auth` command currently does login — removing the `RunE` is a breaking CLI change, but the spec explicitly calls for `auth login` only (no alias). Since this is a developer tool used by its author, the breakage window is small.

**`auth status` token refresh**: Uses `gdrive.EnsureTokenFresh` (already implemented in `pkg/gdrive/auth.go`). If the token is expired but a refresh token is present, the oauth2 library refreshes automatically when a new HTTP client is created. The refreshed token is saved to `token.json` via `saveToken`. This is a side effect of `auth status`, which is acceptable and documented in FR-017.

---

## 8. `doctor` Exit Code

**Decision**: Exit 0 on all-pass or warn-only; exit 1 on any failure.

**Rationale**: Standard UNIX health-check convention. Enables `get-out doctor && get-out export` scripting. Warnings (e.g., optional `people.json` missing, old config dir still present) should not block automation.

**Implementation**: Maintain a `failCount int` counter; after all checks, `if failCount > 0 { os.Exit(1) }`. Cobra's `RunE` error return path is not used here because `doctor` always prints its full report — returning an error from `RunE` would suppress the cobra usage output (which is silenced via `SilenceUsage: true`) but would also print an error message to stderr via cobra's default error handling, which interferes with the styled summary output. Calling `os.Exit(1)` directly after the summary gives full control.

---

## 9. Man Page

**Decision**: Hand-authored `man/get-out.1` in troff format, bundled in release archives and installed by Homebrew cask via `manpages:`.

**Rationale**: A man page is the standard complement to a CLI tool installed via Homebrew. Users expect `man get-out` to work after `brew install`. The page will cover all commands available after this spec is implemented (including the new self-service commands).

**Sections**: NAME, SYNOPSIS, DESCRIPTION, COMMANDS (one `.TP` per command), OPTIONS (global flags), FILES (`~/.get-out/` and its contents), EXAMPLES, AUTHOR.

---

## 11. SecretStore Abstraction and `go-keyring` Selection

**Decision**: Introduce `pkg/secrets` with a `SecretStore` interface backed by either `KeychainStore` (OS keychain via `go-keyring`) or `FileStore` (plain files at 0600). Use `github.com/zalando/go-keyring v0.2.6`.

**Rationale**: `token.json` and `credentials.json` contain long-lived OAuth secrets that should not sit in plaintext at `0644` in the user's home directory. The OS keychain (macOS Keychain, Linux Secret Service) provides encrypted at-rest storage with access control enforced by the OS — no application-level key management required. The `go-keyring` library is pure Go on macOS (uses the Security framework via CGO-free bindings) and Linux (uses Secret Service D-Bus API). It is already in use in the gcal-organizer reference project, ensuring consistency across the jflowers toolchain.

**Library selection rationale** (alternatives considered):

| Library | Verdict | Reason |
|---------|---------|--------|
| `github.com/zalando/go-keyring v0.2.6` | **Selected** | Minimal API (Get/Set/Delete); pure Go on macOS; active maintenance; used in gcal-organizer |
| `github.com/keybase/go-keychain` | Rejected | macOS-only; requires CGO; no Linux support |
| `github.com/99designs/keyring` | Rejected | Heavier dependency (supports many backends we don't need); wraps go-keyring internally |

**Service name**: `com.jflowers.get-out`

**Well-known keys**:
- `oauth-token` — stores the full contents of `token.json`
- `credentials-json` — stores the full contents of `credentials.json`

**Probe strategy**: On startup, `NewStore()` writes a sentinel value to key `__get_out_probe__`, reads it back, then deletes it. If any step fails, the store silently falls back to `FileStore`. This handles headless/CI environments (no keychain daemon), sandboxed environments, and containers gracefully without printing an error to the user.

**`--no-keyring` flag**: A global persistent flag on `rootCmd`. When set, `NewStore()` skips the probe entirely and returns `FileStore` unconditionally. Designed for CI pipelines and headless systems where the probe itself would be slow or noisy.

**`FileStore` key-to-file mapping** (for get-out, no `.env` file needed — only two keys):
- `oauth-token` → `token.json` (0600)
- `credentials-json` → `credentials.json` (0600)

**Migration** (`pkg/secrets/migrate.go`): Called from `runInit`. Idempotent — safe to run on every `init`. For `token.json`: read from disk → write to store → delete file. For `credentials.json`: read from disk → write to store → if `--non-interactive`: print notice ("credentials.json still on disk — delete manually when ready"); if interactive: prompt with `huh.NewConfirm()` → delete on acceptance. Crash-recovery: if secret is already in store AND file exists on disk, re-attempt file deletion (handles crash between store.Set and os.Remove).

**Doctor check 1 update**: Reports `"Secret storage: OS keychain"` or `"Secret storage: plaintext files"` in the check 1 row output. This is always a pass (both backends are valid); the backend name is informational only.

**`pkg/gdrive/auth.go` refactor**: `Authenticate()` and `EnsureTokenFresh()` are refactored to accept `secrets.SecretStore` instead of reading/writing files directly. `loadToken` and `saveToken` become `store.Get(secrets.KeyOAuthToken)` and `store.Set(secrets.KeyOAuthToken, ...)`. Credential bytes come from `store.Get(secrets.KeyClientCredentials)`. The `Config` struct no longer needs `CredentialsPath` or `TokenPath` for the primary flow — those fields are retained for backward compatibility with `doctor` check logic that calls `os.Stat` directly.

**Test strategy for `pkg/secrets`**:
- `KeychainStore` tests: use `keyring.MockInit()` and `keyring.MockInitWithError()` — no real OS keychain required; safe in CI
- `FileStore` tests: use `t.TempDir()` — no real filesystem paths
- `Migrate` tests: use `keyring.MockInit()` + `t.TempDir()`; inject `PromptFunc` to avoid interactive prompts
- `NewStore` probe test: `keyring.MockInitWithError()` forces FileStore fallback; `keyring.MockInit()` forces KeychainStore success

---

## 10. `setup-browser` Step-Skip Logic

**Decision**: If any step fails, all subsequent steps are reported as "Skipped (previous step failed)" and no further network calls are made.

**Rationale**: Steps are sequentially dependent — there is no useful information to be gained from attempting step 4 (credential extraction) if step 3 (Slack tab found) failed. Skipping cleanly avoids confusing secondary failures (e.g., a CDP timeout because no Slack tab is present producing a generic "extraction failed" error when the real issue is "no Slack tab").

**Implementation**: Use a `failed bool` sentinel. At the start of each step, if `failed` is true, print the skip message and continue to the next iteration. This is simpler than `break` and preserves the full 5-step output structure for the user.
