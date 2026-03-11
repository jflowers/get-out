# Feature Specification: Distribution, Config Migration & Self-Service Commands

**Feature Branch**: `003-distribution`
**Created**: 2026-03-11
**Status**: Draft
**Input**: User description: "publish with homebrew and run with the config dir, channels.json etc actually living in ~/.get-out. publish with signed apple binaries. add commands like doctor, auth login, setup-browser, install, and init in the style of gcal-organizer."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - First-Time Setup via Homebrew (Priority: P1)

A new user discovers get-out and wants to install it without cloning the repository or building from source. They install via Homebrew, run `get-out init` to scaffold their configuration directory, authenticate with Google, connect their browser, and run their first export — all without reading source code or manually creating directories.

**Why this priority**: This is the primary new-user acquisition path. Everything else in this spec depends on the tool being installable and self-configuring. A bad first-run experience blocks all downstream value.

**Independent Test**: A user on a fresh macOS machine with Homebrew installed can run `brew tap jflowers/tools && brew install get-out && get-out init && get-out auth login && get-out setup-browser && get-out export` and complete a Slack export to Google Docs without reading source code or manually creating any directories or config files.

**Acceptance Scenarios**:

1. **Given** a macOS machine with Homebrew installed and no prior get-out installation, **When** the user runs `brew tap jflowers/tools && brew install get-out`, **Then** the `get-out` binary is available in PATH and runs without Gatekeeper warnings or quarantine prompts.
2. **Given** get-out is freshly installed and `~/.get-out/` does not exist, **When** the user runs `get-out init`, **Then** `~/.get-out/` is created with mode 0700, template `conversations.json` and `settings.json` are written, the user is interactively prompted for their Google Drive folder ID, and a "Next Steps" summary is printed.
3. **Given** `get-out init` has run and `credentials.json` has been placed in `~/.get-out/`, **When** the user runs `get-out auth login`, **Then** a browser-based OAuth consent flow completes, a token is saved to `~/.get-out/token.json`, and the connected Google account email is printed.
4. **Given** Chrome is running with remote debugging enabled, **When** the user runs `get-out setup-browser`, **Then** the tool walks through five verification steps (port reachable → tabs listed → Slack tab found → credentials extracted → Slack API validated) and prints a pass/warn/fail result for each.
5. **Given** init, auth, and setup-browser have all succeeded, **When** the user runs `get-out export`, **Then** the export uses the folder ID from `~/.get-out/settings.json` without requiring a `--folder-id` flag.

---

### User Story 2 - Existing User Config Migration (Priority: P2)

An existing get-out user has their configuration in `~/.config/get-out/` (the old default location). After upgrading to the new release, they run `get-out init` and their existing configuration files are automatically copied to `~/.get-out/` without losing data or needing to reconfigure from scratch.

**Why this priority**: Existing users must not be broken by this change. Migration must be automatic on `init` to avoid a sharp upgrade cliff. Comes second because new users outnumber existing users over time, but existing users feel breakage immediately.

**Independent Test**: A user with a populated `~/.config/get-out/` directory runs `get-out init` on a build where the default config dir is `~/.get-out/`. All files are copied, a migration notice is printed, and all existing commands continue to work against the new location without any additional steps.

**Acceptance Scenarios**:

1. **Given** `~/.config/get-out/` contains `conversations.json`, `people.json`, `credentials.json`, and `token.json`, and `~/.get-out/` does not exist, **When** the user runs `get-out init`, **Then** all four files are copied to `~/.get-out/`, a migration notice is printed listing each file, and the old directory is left untouched.
2. **Given** migration has run and `~/.get-out/` is fully populated, **When** the user runs any existing command (e.g., `get-out list`, `get-out export`), **Then** the command reads config from `~/.get-out/` and functions identically to before the upgrade.
3. **Given** `~/.get-out/` already exists and is non-empty, **When** the user runs `get-out init` again, **Then** only managed files absent from `~/.get-out/` are copied from the old directory (if it exists); existing files in `~/.get-out/` are never overwritten, and the command completes cleanly.
4. **Given** only `~/.get-out/` exists (no old dir), **When** the user runs `get-out doctor`, **Then** no migration warning is shown.
5. **Given** both `~/.config/get-out/` and `~/.get-out/` exist, **When** the user runs `get-out doctor`, **Then** a warning is printed noting that the old directory still exists and can be deleted.

---

### User Story 3 - Health Check Before Export (Priority: P3)

A user wants to verify their environment is fully configured before running an export. They run `get-out doctor` and see a clear pass/warn/fail report for each prerequisite (config dir, credentials, token, Drive connectivity, conversations file, Chrome connection, Slack session), with inline instructions for any failures.

**Why this priority**: Reduces support burden and user frustration. Without `doctor`, users discover broken configuration through cryptic errors deep in an export run. With `doctor`, they find and fix issues in under a minute.

**Independent Test**: A user with a broken configuration (e.g., expired token, Chrome not running) runs `get-out doctor` and the output clearly identifies every broken prerequisite with a specific corrective action, without starting any export.

**Acceptance Scenarios**:

1. **Given** a fully configured environment (all files present, token valid, Drive reachable, Chrome running with Slack tab), **When** the user runs `get-out doctor`, **Then** all 10 checks show as passed and the summary reads "10/10 passed".
2. **Given** `token.json` is absent, **When** the user runs `get-out doctor`, **Then** the token check shows as failed with the message "Run: get-out auth login".
3. **Given** Chrome is not running on the configured port, **When** the user runs `get-out doctor`, **Then** the Chrome check shows as failed with instructions to launch Chrome with remote debugging enabled.
4. **Given** `conversations.json` exists but contains 0 configured conversations, **When** the user runs `get-out doctor`, **Then** the conversations check shows a warning (not a failure) with instructions to edit the file.
5. **Given** `get-out doctor` is run with `--verbose`, **Then** the full file path for each checked resource is shown inline on each check row.

---

### User Story 4 - Authentication Status and Re-auth (Priority: P3)

A user wants to check whether their Google authentication is still valid without triggering a new OAuth flow. They run `get-out auth status` and see the token validity, expiry date, and connected account email. If the token is expired, they can run `get-out auth login` to re-authenticate.

**Why this priority**: Completes the auth sub-command structure. Enables users and scripts to check auth state programmatically before running exports.

**Independent Test**: A user with a valid token runs `get-out auth status` and sees their Google account email and token expiry without triggering a browser window. A user with an expired token sees an actionable error message directing them to `get-out auth login`.

**Acceptance Scenarios**:

1. **Given** a valid, non-expired token, **When** the user runs `get-out auth status`, **Then** the output shows: credentials present, token valid, expiry date, and the connected Google account email — with no browser window opened.
2. **Given** an expired token with a refresh token present, **When** the user runs `get-out auth status`, **Then** the token is silently refreshed, the output shows the new expiry, and the refreshed token is saved to disk.
3. **Given** no `token.json` file, **When** the user runs `get-out auth status`, **Then** the output shows "Not authenticated — run: get-out auth login" and exits with a non-zero status code.
4. **Given** `credentials.json` is absent, **When** the user runs `get-out auth login`, **Then** the command exits immediately with a clear error message explaining where to obtain credentials and the exact path to place them.

---

### User Story 5 - Browser Setup Wizard (Priority: P4)

A user needs to connect get-out to their running Chrome browser so it can extract Slack credentials for DM and group message exports. They run `get-out setup-browser` and are walked through five steps: verifying Chrome is reachable, listing open tabs, confirming a Slack tab is open, extracting credentials, and validating the Slack session — with a pass/warn/fail result for each step and actionable fix instructions for any failure.

**Why this priority**: Replaces the existing `test` command with a more guided experience. Lower priority because users who already have `test` working are unaffected until they upgrade.

**Independent Test**: A user with Chrome running and a Slack tab open runs `get-out setup-browser` and all 5 steps show as passed, with the Slack workspace name printed. A user without Chrome running sees Step 1 fail with exact launch instructions for macOS.

**Acceptance Scenarios**:

1. **Given** Chrome is running with remote debugging on the default port and a Slack tab is open with an active session, **When** the user runs `get-out setup-browser`, **Then** all 5 steps pass and the Slack workspace name is printed.
2. **Given** Chrome is not running, **When** the user runs `get-out setup-browser`, **Then** Step 1 fails with a printed command the user can copy and paste to launch Chrome with remote debugging, and steps 2–5 are skipped.
3. **Given** Chrome is running but no Slack tab is open, **When** the user runs `get-out setup-browser`, **Then** steps 1–2 pass, step 3 warns "No Slack tab found — navigate to app.slack.com", and steps 4–5 are skipped.
4. **Given** the user specifies a non-default Chrome port with `--chrome-port 9223`, **When** the user runs `get-out setup-browser`, **Then** the wizard checks that port instead of the default.

---

### Edge Cases

- What happens when `get-out init` is run as root (effective UID 0)? The config dir should still be created under the actual user's home, not `/root`.
- What happens when `~/.get-out/` exists but is a file rather than a directory? `init` must detect this and exit with a clear error rather than failing silently.
- What happens when `conversations.json` exists in the old dir but is corrupt JSON? Migration copies it as-is; `doctor` will subsequently detect and report the parse failure.
- What happens when the Google Drive folder ID entered during `init` is syntactically invalid? The prompt must validate the format before accepting it.
- What happens when the Homebrew cask is installed on a machine that already has get-out installed via `go install`? Standard Homebrew behaviour applies (binary in Homebrew's bin takes precedence in PATH).
- What happens when the macOS signing certificate has expired? The release workflow must detect signing failure and not publish broken assets; the `sign-macos` CI job must exit non-zero and block the Homebrew cask update.
- What happens when `get-out setup-browser` is run on Linux (where macOS launch instructions are irrelevant)? Chrome launch instructions must be OS-appropriate.
- What happens when both `~/.get-out/settings.json` has a `folder_id` AND `--folder-id` is passed on the command line? The command-line flag wins.
- What happens if `credentials.json` or `token.json` already exist in `~/.get-out/` with world-readable permissions (e.g., copied manually)? `get-out doctor` MUST warn if either file has permissions broader than `0600`.

## Requirements *(mandatory)*

### Functional Requirements

**Config Directory**

- **FR-001**: The default configuration directory MUST be `~/.get-out/` (resolved from the current user's home directory).
- **FR-002**: All configuration files (`settings.json`, `conversations.json`, `people.json`, `credentials.json`, `token.json`) MUST be read from and written to the configured directory.
- **FR-003**: The `--config` flag MUST continue to override the default directory for all commands, enabling development and multi-account use.
- **FR-004**: `settings.json` MUST support a `folder_id` field that `export` reads as the default Google Drive destination when `--folder-id` is not specified on the command line.

**`init` Command**

- **FR-005**: `get-out init` MUST create `~/.get-out/` with mode 0700 if it does not exist. `credentials.json` and `token.json` MUST be written (or copied during migration) with mode 0600 (owner read/write only). All other files in the config directory use the process umask default.
- **FR-006**: `get-out init` MUST write a minimal template `conversations.json` (`{"conversations":[]}`) if the file does not already exist in the target directory.
- **FR-007**: `get-out init` MUST write a minimal template `settings.json` (`{}`) if the file does not already exist.
- **FR-008**: `get-out init` MUST detect whether `~/.config/get-out/` exists and, if so, copy each managed file (`settings.json`, `conversations.json`, `people.json`, `credentials.json`, `token.json`) from the old directory to `~/.get-out/` only if that file does not already exist in `~/.get-out/`. A notice MUST be printed for each file copied. Files already present in `~/.get-out/` are never overwritten by migration.
- **FR-009**: `get-out init` MUST NOT delete the old `~/.config/get-out/` directory; it MUST print a notice that the old directory can be removed manually.
- **FR-010**: `get-out init` MUST interactively prompt for the Google Drive folder ID when `--non-interactive` is not set and no `folder_id` is present in `settings.json`. When `--non-interactive` is set and `folder_id` is absent, `init` MUST succeed silently and leave `folder_id` unset; in that case `export` will continue to require the `--folder-id` flag at runtime.
- **FR-011**: The folder ID prompt MUST validate that the entered value matches the format of a Google Drive ID (28 or more alphanumeric, hyphen, or underscore characters) before accepting it.
- **FR-012**: On success, `get-out init` MUST print a "Next Steps" summary listing the remaining setup actions in order.

**`auth` Sub-Commands**

- **FR-013**: `get-out auth` MUST be a command group; running it without a sub-command MUST print help.
- **FR-014**: `get-out auth login` MUST perform the full Google OAuth 2.0 browser consent flow and save the resulting token to `token.json`.
- **FR-015**: `get-out auth login` MUST exit with a clear, actionable error message if `credentials.json` is not present before attempting the OAuth flow.
- **FR-016**: `get-out auth status` MUST report, without opening a browser: whether `credentials.json` is present, whether `token.json` is present, whether the token is valid and its expiry date, and the Google account email if the token is valid.
- **FR-017**: `get-out auth status` MUST attempt a silent token refresh if the token is expired but a refresh token is present, save the refreshed token, and report the new expiry.
- **FR-018**: `get-out auth status` MUST exit with a non-zero status code when the token is absent or cannot be refreshed.

**`doctor` Command**

- **FR-019**: `get-out doctor` MUST execute the following 10 checks in order and print a styled pass/warn/fail result for each: (1) config dir exists with correct permissions, (2) `credentials.json` present, (3) `token.json` present, (4) token valid or refreshable, (5) Google Drive API reachable — verified by making an authenticated `drive.About.Get` call (exercises both network and credential validity), (6) `conversations.json` present and parseable, (7) `people.json` present, (8) Chrome reachable on configured port, (9) Slack tab found in Chrome, (10) `export-index.json` present and readable.
- **FR-020**: Each failing check in `get-out doctor` MUST print a specific corrective action (e.g., command to run or file to create).
- **FR-021**: `get-out doctor` MUST print a summary line showing counts of passed, warned, and failed checks.
- **FR-021a**: `get-out doctor` MUST exit with code 0 when all checks pass or produce warnings only; it MUST exit with code 1 when one or more checks fail. This enables scripting patterns such as `get-out doctor && get-out export`.
- **FR-022**: When `~/.config/get-out/` still exists alongside `~/.get-out/`, `get-out doctor` MUST display a warning suggesting the old directory can be deleted.
- **FR-023**: `get-out doctor --verbose` MUST show the resolved file path for each checked resource inline on its check row.
- **FR-023a**: `get-out doctor` MUST warn if `credentials.json` or `token.json` exist with file permissions broader than `0600` (e.g., group- or world-readable), and MUST suggest `chmod 0600 <path>` as the corrective action.

**`setup-browser` Command**

- **FR-024**: `get-out setup-browser` MUST replace the `test` command; the `test` command MUST be removed.
- **FR-025**: `get-out setup-browser` MUST execute five steps in order: (1) verify Chrome is reachable on the configured port, (2) list open page tabs, (3) identify a Slack tab using the same URL validation used by the rest of the tool, (4) extract Slack credentials, (5) validate the extracted credentials against the Slack API.
- **FR-026**: If any step fails, all subsequent steps MUST be skipped and reported as "Skipped".
- **FR-027**: `get-out setup-browser` MUST display Chrome launch instructions appropriate to the user's operating system when step 1 fails.
- **FR-028**: Slack credentials displayed by `get-out setup-browser` MUST be masked (first 15 characters + "..." + last 4 characters) to prevent accidental exposure.

**Distribution**

- **FR-029**: Releases MUST produce signed and notarized macOS binaries for both `darwin/amd64` and `darwin/arm64` targets.
- **FR-030**: Signed macOS binaries MUST pass Gatekeeper validation on a stock macOS machine without requiring `xattr -d com.apple.quarantine`.
- **FR-031**: A Homebrew cask MUST be published to the `jflowers/homebrew-tools` tap on each release, with correct SHA256 checksums reflecting the signed binaries.
- **FR-032**: Users MUST be able to install get-out with `brew tap jflowers/tools && brew install get-out`.
- **FR-033**: Releases MUST also include `linux/amd64` and `linux/arm64` builds in the Homebrew cask.
- **FR-034**: Each release archive MUST include a man page (`man/get-out.1`) installed by the Homebrew cask.
- **FR-035**: The release pipeline MUST degrade gracefully: if signing secrets are absent, binaries MUST still be published unsigned and the Homebrew cask update MUST be skipped rather than failing the whole release.

### Key Entities

- **Config Directory** (`~/.get-out/`): The single canonical location for all user-specific state. Contains configuration files, OAuth credentials, OAuth token, and the export index. Mode 0700 (user-only access).
- **Settings** (`settings.json`): Key–value store for non-secret application preferences, including the default Google Drive folder ID. Optional; defaults applied when absent or when fields are missing.
- **Conversations Config** (`conversations.json`): Ordered list of Slack conversations to export, each with an ID, name, type, mode, and export flag. Required for `list` and `export` commands.
- **People Config** (`people.json`): Optional mapping of Slack user IDs to display names and Google emails. Created by `get-out discover`.
- **OAuth Credentials** (`credentials.json`): Google Cloud OAuth 2.0 client credentials. Must be obtained manually from Google Cloud Console and placed in the config directory.
- **OAuth Token** (`token.json`): Access and refresh tokens obtained via `get-out auth login`. Managed entirely by the tool.
- **Export Index** (`export-index.json`): Checkpoint file tracking export progress and linking Slack conversations to Google Drive documents. Written and read by `get-out export` and `get-out status`.
- **Homebrew Tap** (`jflowers/homebrew-tools`): Public GitHub repository hosting Homebrew cask definitions for jflowers tools. Updated automatically by the release pipeline.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A user on a fresh macOS machine completes the full setup sequence (`brew install`, `init`, `auth login`, `setup-browser`) and runs a successful export in under 10 minutes, following only the instructions printed by the tool itself.
- **SC-002**: `get-out doctor` identifies every misconfiguration (missing file, expired token, Chrome not running) in a single run and provides a corrective action for each failure, with no export required.
- **SC-003**: Signed macOS binaries pass Gatekeeper on a stock macOS machine with no manual quarantine removal required — verified by downloading from the GitHub release and running without `xattr`.
- **SC-004**: After running `get-out init` on a machine with an existing `~/.config/get-out/` directory, all previously working commands (`list`, `export`, `status`) function identically against `~/.get-out/` without any manual file moves or flag changes.
- **SC-005**: `get-out auth status` completes in under 3 seconds, including a silent token refresh if needed, with no browser window opened.
- **SC-006**: The Homebrew cask SHA256 checksums in `jflowers/homebrew-tools` match the signed archives published to the GitHub release within 30 minutes of a tag being pushed.
- **SC-007**: `get-out setup-browser` provides OS-appropriate Chrome launch instructions — macOS instructions on macOS, Linux instructions on Linux — without requiring the user to know which OS they are on.

## Clarifications

### Session 2026-03-11

- Q: What should `get-out doctor`'s process exit code be? → A: Exit 0 if all pass or warn-only; exit 1 if any check fails.
- Q: What should `get-out init --non-interactive` do when `folder_id` is absent? → A: Succeed silently; leave `folder_id` unset in `settings.json`.
- Q: What constitutes "Google Drive API reachable" in `doctor` check #5? → A: An authenticated `drive.About.Get` call — exercises both network and credential validity.
- Q: What file permissions should sensitive credential files have? → A: `credentials.json` and `token.json` written with mode 0600; other files use process umask default.
- Q: When `~/.get-out/` is non-empty and the old dir also has files, how should migration proceed? → A: Copy only files not already present in `~/.get-out/`; never overwrite existing files.

## Assumptions

- The Apple Developer ID certificate (`Developer ID Application: John Flowers, PGFWLVZX55`) and App Store Connect notarization key (`4K669B7BD9`) used by gcal-organizer are the same credentials used for get-out signing — no new Apple developer enrollment is needed.
- The `HOMEBREW_TAP_TOKEN` GitHub secret (a PAT with write access to `jflowers/homebrew-tools`) is the same token used by gcal-organizer for its tap; the same secret value is set in the get-out repo.
- The `jflowers/homebrew-tools` GitHub repository must be created before the first release; the tap name exposed to users is `jflowers/tools` (Homebrew convention strips the `homebrew-` prefix).
- Linux builds are included in the Homebrew cask for completeness; macOS signing and notarization apply only to darwin builds.
- No man page exists yet; it will be hand-authored as part of this feature. Content covers all commands present after this spec is implemented.
- The existing `test` command and its test file are removed as part of this feature; `safePreview` is moved to a shared helpers file so existing unit tests continue to compile.
- Interactive prompts in `init` use `charmbracelet/huh`; styled output in `doctor` and `setup-browser` uses `charmbracelet/lipgloss` — the same Charm stack used in gcal-organizer.
- `get-out install` (background service installation) is out of scope for this spec. get-out is an on-demand CLI tool, not a daemon; launchd/systemd integration adds complexity without clear user value at this stage.
