## ADDED Requirements

### Requirement: Local Markdown Output

The export command MUST write a local markdown file for each conversation-date combination when both conditions are met:
1. `localExportOutputDir` is configured in settings or `--local-export-dir` is provided via CLI flag (global enablement)
2. The conversation has `localExport: true` in `conversations.json` (per-conversation opt-in)

Each markdown file MUST contain:
- YAML frontmatter with `conversation`, `type`, `date`, and `participants` fields
- Message content formatted as standard markdown with timestamps and sender names
- Preserved formatting: bold (`**text**`), italic (`*text*`), strikethrough (`~~text~~`), inline code, fenced code blocks, and hyperlinks

Each markdown file SHALL be named `{date}.md` (e.g., `2026-01-15.md`) and placed in a subdirectory named `{type-prefix}-{sanitized-name}` under the configured export directory.

#### Scenario: Basic daily export with local markdown enabled

- **GIVEN** `localExportOutputDir` is set to `~/.get-out/export` in settings.json
- **AND** a conversation "John Smith" (DM, ID D12345) has `localExport: true` in conversations.json
- **AND** the conversation has 5 messages on 2026-01-15
- **WHEN** the user runs `get-out export D12345`
- **THEN** a Google Doc titled "2026-01-15" is created in Drive (existing behavior)
- **AND** a file `~/.get-out/export/dm-john-smith/2026-01-15.md` is created
- **AND** the file contains YAML frontmatter with `conversation: John Smith`, `type: dm`, `date: 2026-01-15`, and `participants` listing the sender names
- **AND** the file contains 5 message entries with timestamps, sender names, and formatted content

#### Scenario: CLI flag overrides config setting

- **GIVEN** `localExportOutputDir` is set to `~/.get-out/export` in settings.json
- **AND** conversation D12345 has `localExport: true`
- **WHEN** the user runs `get-out export --local-export-dir /tmp/slack-export D12345`
- **THEN** markdown files are written to `/tmp/slack-export/dm-john-smith/` instead of `~/.get-out/export/dm-john-smith/`

#### Scenario: Local export disabled by default (no directory configured)

- **GIVEN** `localExportOutputDir` is not set in settings.json
- **AND** `--local-export-dir` is not provided
- **WHEN** the user runs `get-out export D12345`
- **THEN** only Google Docs are created (existing behavior)
- **AND** no local markdown files are written

#### Scenario: Conversation not opted in for local export

- **GIVEN** `localExportOutputDir` is set to `~/.get-out/export` in settings.json
- **AND** conversation D12345 has `localExport: false` (or field is omitted, defaulting to false)
- **WHEN** the user runs `get-out export D12345`
- **THEN** a Google Doc is created in Drive (existing behavior)
- **AND** no local markdown file is written for this conversation

#### Scenario: Mixed conversations with different local export settings

- **GIVEN** `localExportOutputDir` is set to `~/.get-out/export` in settings.json
- **AND** conversation "design-decisions" (channel) has `localExport: true`
- **AND** conversation "John Smith" (DM) has `localExport: false`
- **WHEN** the user runs `get-out export`
- **THEN** both conversations are exported to Google Docs
- **AND** only `~/.get-out/export/channel-design-decisions/` contains markdown files
- **AND** no `dm-john-smith/` directory is created

#### Scenario: localExport true but Export false (no-op)

- **GIVEN** `localExportOutputDir` is set to `~/.get-out/export` in settings.json
- **AND** conversation D12345 has `localExport: true` but `export: false`
- **WHEN** the user runs `get-out export`
- **THEN** conversation D12345 is not exported at all (messages are not fetched)
- **AND** no local markdown file is written for this conversation
- **AND** no warning is logged (this is expected behavior -- `export: false` means the conversation is excluded from all export)

#### Scenario: Dry-run with local export configured

- **GIVEN** `localExportOutputDir` is set to `~/.get-out/export` in settings.json
- **AND** conversation D12345 has `localExport: true`
- **WHEN** the user runs `get-out export --dry-run D12345`
- **THEN** no Google Docs are created (existing dry-run behavior)
- **AND** no local markdown files are written
- **AND** the dry-run output indicates that local markdown would be written to `~/.get-out/export/dm-john-smith/`

### Requirement: Per-Conversation Local Export Opt-In

The `ConversationConfig` struct MUST include a `localExport` boolean field (JSON key `"localExport"`). This field MUST default to `false` when omitted from `conversations.json`. Local markdown export for a conversation SHALL only occur when this field is explicitly set to `true`.

#### Scenario: Default value when field is omitted

- **GIVEN** a conversation entry in `conversations.json` without a `localExport` field
- **WHEN** the configuration is loaded
- **THEN** `LocalExport` is `false`
- **AND** no local markdown is written for this conversation even if `localExportOutputDir` is configured

#### Scenario: Explicit opt-in

- **GIVEN** a conversation entry with `"localExport": true` in `conversations.json`
- **AND** `localExportOutputDir` is configured
- **WHEN** the conversation is exported
- **THEN** local markdown files are written for this conversation

### Requirement: Markdown Formatting Conversion

The parser MUST provide a `ConvertMrkdwnToMarkdown` function that converts Slack mrkdwn to standard markdown, preserving formatting markers rather than stripping them.

The conversion MUST handle:
- `*bold*` → `**bold**`
- `_italic_` → `*italic*`
- `~strikethrough~` → `~~strikethrough~~`
- `` `inline code` `` → `` `inline code` `` (preserved)
- ` ```code block``` ` → fenced code block with triple backticks
- `<https://url|display text>` → `[display text](https://url)`
- `<https://url>` → `<https://url>` (autolink)
- `<@U12345>` → resolved display name (same resolution as existing)
- `<#C12345|channel-name>` → `#channel-name`
- `<!here>`, `<!channel>`, `<!everyone>` → `@here`, `@channel`, `@everyone`

Formatting markers inside code spans or fenced code blocks MUST be preserved literally (not converted).

#### Scenario: Bold and italic conversion

- **GIVEN** a Slack message with text `*important* _note_ ~removed~`
- **WHEN** the message is converted via `ConvertMrkdwnToMarkdown`
- **THEN** the output is `**important** *note* ~~removed~~`

#### Scenario: Link with display text

- **GIVEN** a Slack message with text `Check <https://example.com|this doc>`
- **WHEN** the message is converted via `ConvertMrkdwnToMarkdown`
- **THEN** the output is `Check [this doc](https://example.com)`

#### Scenario: Code block conversion

- **GIVEN** a Slack message with text `` ```func main() { fmt.Println("hello") }``` ``
- **WHEN** the message is converted via `ConvertMrkdwnToMarkdown`
- **THEN** the output contains a fenced code block with triple backticks

#### Scenario: Formatting inside code spans is preserved literally

- **GIVEN** a Slack message with text `` `*not bold* _not italic_` ``
- **WHEN** the message is converted via `ConvertMrkdwnToMarkdown`
- **THEN** the output is `` `*not bold* _not italic_` `` (formatting markers are not converted)

#### Scenario: Nested formatting

- **GIVEN** a Slack message with text `*bold with _italic_ inside*`
- **WHEN** the message is converted via `ConvertMrkdwnToMarkdown`
- **THEN** the output preserves both formatting markers: `**bold with *italic* inside**`

#### Scenario: Empty input

- **GIVEN** a Slack message with empty text `""`
- **WHEN** the message is converted via `ConvertMrkdwnToMarkdown`
- **THEN** the output is `""`

### Requirement: YAML Frontmatter

Each exported markdown file MUST begin with YAML frontmatter delimited by `---` containing:
- `conversation`: the conversation name (string)
- `type`: the conversation type using the canonical `models.ConversationType` value (`dm`, `mpim`, `channel`, `private_channel`)
- `date`: the date in `YYYY-MM-DD` format (string)
- `participants`: list of unique sender display names appearing in that day's messages

#### Scenario: Frontmatter content

- **GIVEN** a DM conversation "Alice" with messages from alice and bob on 2026-03-01
- **WHEN** the daily markdown file is generated
- **THEN** the file begins with:
  ```
  ---
  conversation: Alice
  type: dm
  date: "2026-03-01"
  participants:
    - alice
    - bob
  ---
  ```

#### Scenario: Private channel frontmatter uses canonical type

- **GIVEN** a private channel conversation "hr-discussions"
- **WHEN** the daily markdown file is generated
- **THEN** the frontmatter contains `type: private_channel` (canonical model value with underscore)

### Requirement: Thread Reply Rendering

Thread replies MUST be included inline within the parent conversation's daily markdown file. Each thread MUST be rendered as:
1. The parent message rendered normally (with timestamp and sender)
2. A "Thread replies:" marker line immediately after the parent message content
3. Each reply rendered as a blockquoted message with timestamp and sender

#### Scenario: Message with thread replies

- **GIVEN** a conversation with a parent message from alice at 9:15 AM saying "Should we use OAuth2?" and 2 thread replies: bob at 9:18 AM saying "Agreed" and alice at 9:20 AM saying "Let's do it"
- **WHEN** the daily markdown file is generated
- **THEN** the output contains the parent message followed by:
  ```
  **Thread replies:**

  > **9:18 AM -- bob**
  >
  > Agreed

  > **9:20 AM -- alice**
  >
  > Let's do it
  ```

### Requirement: Atomic File Writes

Markdown files MUST be written atomically using a write-to-temp-then-rename pattern. The writer MUST:
1. Create a temporary file with a `.tmp-` prefix in the same directory as the target
2. Write the full content to the temporary file
3. Call `os.Rename` to atomically move the temp file to the final path
4. If the write fails, remove the temporary file before returning the error

The `.tmp-` prefix ensures temporary files are not indexed by Dewey (which indexes `*.md` files).

#### Scenario: Atomic write implementation

- **GIVEN** a `WriteMarkdownFile` call for `2026-01-15.md`
- **WHEN** the content is written
- **THEN** a temporary file `.tmp-{random}.md` is created in the target directory
- **AND** the full content is written to the temp file
- **AND** `os.Rename` atomically moves it to `2026-01-15.md`
- **AND** no partially-written `2026-01-15.md` is ever observable

#### Scenario: Temp file cleanup on write failure

- **GIVEN** a `WriteMarkdownFile` call where the write to the temp file fails (e.g., disk full)
- **WHEN** the error is returned
- **THEN** the temporary file is removed (best-effort)
- **AND** no file exists at the target path

### Requirement: Non-Fatal Markdown Write Errors

Markdown write failures MUST NOT abort the export. If writing a markdown file fails, the error SHOULD be logged via the progress callback and export MUST continue with the Google Docs output.

The export summary MUST include a count of local markdown files written and a count of markdown write failures when local export is enabled.

#### Scenario: Disk full during local export

- **GIVEN** `localExportOutputDir` is configured and the target disk is full
- **WHEN** the export attempts to write a markdown file and receives an OS error
- **THEN** the error is logged as a warning via the progress callback
- **AND** the Google Docs export for that date continues normally
- **AND** the export proceeds to the next date
- **AND** the export summary includes the failure count

### Requirement: Incremental Export (Skip Logic)

When the export loop processes a conversation-date combination, the markdown writer MUST check if the target markdown file already exists on disk before writing. If the file exists, the write MUST be skipped. This file-existence check is independent of the checkpoint index.

This means:
- If a user deletes a markdown file, it will be regenerated on the next export run that processes that date (unlike the checkpoint-index-driven Google Docs behavior).
- If a date was already exported to Google Docs and the checkpoint index skips it, no markdown write attempt occurs (the export loop does not re-process that date).

#### Scenario: Existing markdown file is skipped

- **GIVEN** `~/.get-out/export/dm-john-smith/2026-01-15.md` already exists
- **AND** the export loop processes date 2026-01-15 for this conversation
- **WHEN** the markdown writer checks the target path
- **THEN** the write is skipped
- **AND** the existing file is not overwritten

#### Scenario: Deleted markdown file is regenerated

- **GIVEN** the checkpoint index has an entry for DM "John Smith" on 2026-01-15
- **AND** the user has deleted `~/.get-out/export/dm-john-smith/2026-01-15.md`
- **AND** the user runs `get-out export` without `--sync` (full re-export)
- **WHEN** the export loop processes date 2026-01-15
- **THEN** `~/.get-out/export/dm-john-smith/2026-01-15.md` is regenerated

#### Scenario: Sync mode skips checkpoint-indexed dates

- **GIVEN** the checkpoint index shows the last exported message for DM "John Smith" was on 2026-01-15
- **AND** new messages exist on 2026-01-16
- **WHEN** the user runs `get-out export --sync D12345`
- **THEN** only 2026-01-16 is processed by the export loop
- **AND** only `2026-01-16.md` is written (or skipped if it already exists)
- **AND** `2026-01-15.md` is not touched (the export loop does not re-process that date)

### Requirement: Directory Naming Convention

Conversation subdirectories MUST be named `{type-prefix}-{sanitized-name}` where:

The type prefix is mapped from `models.ConversationType` to a filesystem-friendly label:

| `ConversationType` | Directory Prefix |
|---|---|
| `dm` | `dm` |
| `mpim` | `group` |
| `channel` | `channel` |
| `private_channel` | `private-channel` |

The `sanitized-name` is the conversation name:
- Converted to lowercase
- Spaces replaced by hyphens
- Non-alphanumeric characters (except hyphens) removed
- Consecutive hyphens collapsed to a single hyphen
- Truncated to 100 characters (to avoid filesystem path length limits)
- If empty after sanitization, the conversation ID is used as fallback

The resulting name MUST be valid on macOS, Linux, and Windows filesystems.

#### Scenario: DM conversation directory name

- **GIVEN** a DM conversation with name "John Smith"
- **WHEN** the export directory is created
- **THEN** the subdirectory is named `dm-john-smith`

#### Scenario: Channel with special characters

- **GIVEN** a channel named "design-decisions (Q1)"
- **WHEN** the export directory is created
- **THEN** the subdirectory is named `channel-design-decisions-q1`

#### Scenario: MPIM group conversation

- **GIVEN** an MPIM conversation with name "Alice, Bob, Carol"
- **WHEN** the export directory is created
- **THEN** the subdirectory is named `group-alice-bob-carol`

#### Scenario: Private channel directory name

- **GIVEN** a private channel conversation with name "hr-discussions"
- **WHEN** the export directory is created
- **THEN** the subdirectory is named `private-channel-hr-discussions`

#### Scenario: Empty name after sanitization

- **GIVEN** a channel named "!!!"
- **WHEN** the export directory is created
- **THEN** the subdirectory uses the conversation ID as fallback (e.g., `channel-C12345ABC`)

### Requirement: Export Directory Path Validation

The export command MUST validate the local export directory path before beginning the export:
1. Only `~/` (current user home) MUST be expanded. Paths starting with `~username/` MUST be rejected with an error.
2. Relative paths MUST be resolved to absolute paths.
3. The path MUST contain at least 2 components (rejecting `/` as a root-level export directory).
4. If the directory does not exist, it MUST be created.
5. If creation fails, the export MUST fail with a clear error message before any Google Docs operations begin.

#### Scenario: Tilde expansion for current user

- **GIVEN** `localExportOutputDir` is set to `~/slack-export`
- **WHEN** the export runs
- **THEN** files are written to `$HOME/slack-export/`, not a literal `~` directory

#### Scenario: Tilde with username is rejected

- **GIVEN** `--local-export-dir ~otheruser/exports` is provided
- **WHEN** the export starts
- **THEN** the export fails with an error: "local export path must use ~/ for current user home, not ~username/"

#### Scenario: Root path is rejected

- **GIVEN** `--local-export-dir /` is provided
- **WHEN** the export starts
- **THEN** the export fails with an error indicating the path is too shallow

#### Scenario: Unwritable directory fails before export

- **GIVEN** `--local-export-dir /root/no-access` is provided and the user does not have write permission
- **WHEN** the export starts
- **THEN** the export fails with an error indicating the directory is not writable
- **AND** no Google Docs operations have started

### Requirement: MarkdownWriter Testability

The `MarkdownWriter` MUST be testable in isolation without filesystem access or external service calls. It SHALL accept message data and resolver interfaces, and return `[]byte` output. Filesystem operations (directory creation, atomic writes) MUST be handled by the caller.

#### Scenario: Unit test without filesystem

- **GIVEN** a `MarkdownWriter` instantiated with mock resolvers
- **WHEN** `RenderDailyDoc` is called with a conversation name, date, and message slice
- **THEN** it returns `[]byte` containing valid markdown with YAML frontmatter
- **AND** no filesystem operations occur

### Requirement: File Permissions

Markdown files MUST be written with mode `0644` (owner read/write, group/others read-only). Directories MUST be created with mode `0755`. These permissions are consistent with non-secret data files in the user's home directory. Conversation content is considered non-secret for permission purposes -- access control is handled by the user's filesystem permissions on the parent directory.

## MODIFIED Requirements

### Requirement: Export Command Flag Set

The `export` command MUST accept a `--local-export-dir` flag that specifies the directory for local markdown output. When provided, this flag overrides the `localExportOutputDir` value in `settings.json`.

Previously: The export command had no local output flags.

### Requirement: Settings Schema

The `Settings` struct `localExportOutputDir` field (already present, marked "for future use") becomes active. When non-empty, it enables the local markdown export directory. Individual conversations still require `localExport: true` to produce markdown output. The path MUST support `~/` to reference the user's home directory (see Export Directory Path Validation).

Previously: `localExportOutputDir` was declared but unused.

### Requirement: Conversation Configuration Schema

The `ConversationConfig` struct MUST include a `LocalExport bool` field (JSON key `"localExport"`, default `false`). This field controls per-conversation opt-in for local markdown export, independent of the `Export` field which controls Google Docs export.

Previously: `ConversationConfig` had no field for local export control.

### Requirement: Dry-Run Output

The `--dry-run` output MUST include local markdown export information when `localExportOutputDir` is configured. For each conversation with `localExport: true`, the dry-run output MUST show the target directory path where markdown files would be written.

Previously: `--dry-run` only showed Google Docs export information.

## REMOVED Requirements

None.
