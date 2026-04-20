Within each group, test tasks SHOULD be implemented before their corresponding implementation tasks per the TDD principle in the constitution (Development Workflow: "Test-Driven: Write tests for API response parsing before implementation").

## 1. Markdown Conversion (Parser)

- [x] 1.1 Add table-driven unit tests for `ConvertMrkdwnToMarkdown` in `pkg/parser/mrkdwn_test.go` covering: bold, italic, strikethrough, inline code, fenced code blocks, links with display text, bare URLs, user mentions, channel mentions, special mentions (@here, @channel, @everyone), HTML entity decoding, formatting inside code spans (must be preserved literally), nested formatting (bold containing italic), empty text, and whitespace-only text
- [x] 1.2 Add `ConvertMrkdwnToMarkdown` function in `pkg/parser/mrkdwn.go` that converts Slack mrkdwn to standard markdown, preserving formatting: `*bold*` → `**bold**`, `_italic_` → `*italic*`, `~strike~` → `~~strike~~`, code blocks to fenced blocks, `<url|text>` → `[text](url)`, user/channel mentions resolved to names; formatting markers inside code spans/blocks MUST be preserved literally

## 2. Markdown Writer Component

- [x] 2.1 Add unit tests for `MarkdownWriter` in `pkg/exporter/mdwriter_test.go` covering: frontmatter correctness (conversation, type, date, participants), message ordering (oldest first), participant extraction from unique senders, empty message handling, multi-sender conversations, reaction formatting, thread reply rendering (blockquoted with "Thread replies:" marker), and directory name sanitization (including type prefix mapping: `dm`→`dm`, `mpim`→`group`, `channel`→`channel`, `private_channel`→`private-channel`, empty-after-sanitization fallback to conversation ID, truncation at 100 chars)
- [x] 2.2 Create `pkg/exporter/mdwriter.go` with `MarkdownWriter` struct that accepts resolver context (UserResolver, ChannelResolver, PersonResolver) and provides `RenderDailyDoc(convName string, convType string, date string, messages []slackapi.Message) ([]byte, error)` returning complete markdown with YAML frontmatter
- [x] 2.3 Implement YAML frontmatter generation with `conversation`, `type` (canonical `ConversationType` value), `date`, and `participants` fields (participants derived from unique sender names in the message slice)
- [x] 2.4 Implement message rendering: each message formatted with bold sender/time line (`**{time} -- {sender}**`) followed by content produced by `ConvertMrkdwnToMarkdown`, reactions appended, attachments formatted as quoted text, and thread replies rendered as blockquoted messages under a "**Thread replies:**" marker
- [x] 2.5 Implement `SanitizeDirectoryName(convType, convName string) string` helper that maps `ConversationType` to directory prefix (`dm`→`dm`, `mpim`→`group`, `channel`→`channel`, `private_channel`→`private-channel`), sanitizes the name to kebab-case, truncates to 100 characters, and falls back to conversation ID if empty after sanitization

## 3. Filesystem Integration

- [x] 3.1 Add unit tests for `WriteMarkdownFile` in `pkg/exporter/mdfile_test.go` covering: directory creation, atomic write with `.tmp-` prefix (verify no partial files at target path), temp file cleanup on write failure, overwrite skipped when target exists (file-existence check), tilde expansion (`~/` → home dir), `~username/` rejection, root path (`/`) rejection, and file permissions (0644 for files, 0755 for directories)
- [x] 3.2 Create `pkg/exporter/mdfile.go` with `WriteMarkdownFile(dir string, convType string, convName string, date string, content []byte) error` that handles: directory creation (`os.MkdirAll` with 0755), file-existence skip check, atomic write (`.tmp-` prefixed temp file + `os.Rename`), temp file cleanup on failure, and file permissions (0644)
- [x] 3.3 Implement `ExpandAndValidatePath(path string) (string, error)` that expands `~/` to the current user's home directory, rejects `~username/` paths, resolves relative paths to absolute, and rejects paths with fewer than 2 components

## 4. Per-Conversation Config

- [x] 4.1 Add unit test in `pkg/config/config_test.go` verifying: `localExport` defaults to `false` when omitted from JSON, `true` when explicitly set, and `false` when explicitly set to `false`
- [x] 4.2 Add `LocalExport bool` field to `ConversationConfig` in `pkg/config/types.go` with JSON tag `"localExport"` (defaults to `false` when omitted)

## 5. Export Loop Integration

- [x] 5.1 Add `mdWriter *MarkdownWriter` and `localExportDir string` fields to the `Exporter` struct; initialize `MarkdownWriter` in `InitializeWithStore` (after resolvers are created) when `localExportDir` is non-empty
- [x] 5.2 In `ExportConversation`, inside the daily-doc iteration after the `docWriter.WriteMessages()` call and checkpoint save, call `MarkdownWriter.RenderDailyDoc` and `WriteMarkdownFile` only when both `localExportDir` is configured AND `conv.LocalExport == true`; check file existence before writing (skip if exists); log errors via progress callback but do not return them (non-fatal); track markdown write success/failure counts in `ExportResult`
- [x] 5.3 Add integration tests in `pkg/exporter/exporter_core_test.go` verifying: (a) markdown files are written when both `localExportDir` is set and `conv.LocalExport` is true — assert file exists at expected path (`{exportDir}/{type}-{name}/{date}.md`), contains valid YAML frontmatter with correct fields, and contains formatted message content; (b) markdown files are not written when `localExportDir` is empty; (c) markdown files are not written when `conv.LocalExport` is false even if `localExportDir` is set; (d) markdown write errors do not abort the export and the progress callback receives a warning; (e) existing markdown files are not overwritten (file-existence skip)

## 6. CLI and Config Wiring

- [x] 6.1 Add `--local-export-dir` flag to the `export` command in `internal/cli/export.go`; wire the flag to override `settings.LocalExportOutputDir` when provided
- [x] 6.2 Call `ExpandAndValidatePath` on the resolved export directory path before constructing the `Exporter`; fail with a clear error message if validation fails (before any Google Docs operations)
- [x] 6.3 Pass the validated `localExportDir` value through to `Exporter` construction so it reaches the `MarkdownWriter` initialization
- [x] 6.4 Update `--dry-run` output to include local markdown export information when `localExportDir` is configured: show which conversations have `localExport: true` and the target directory paths

## 7. Documentation

- [x] 7.1 Update `README.md` with: new `--local-export-dir` flag documentation, `localExportOutputDir` settings field, `localExport` conversation field with example `conversations.json`, example Dewey `sources.yaml` configuration, and example markdown output
- [x] 7.2 Update `AGENTS.md` with: `pkg/exporter/mdwriter.go` and `pkg/exporter/mdfile.go` in the project structure

## 8. Verification

- [x] 8.1 Run `go test -race -count=1 ./...` and verify all tests pass
- [x] 8.2 Run `go build -o get-out ./cmd/get-out` and verify the binary builds
- [x] 8.3 Run `go vet ./...` and verify no issues
- [x] 8.4 Run `go test -race -count=1 -coverprofile=coverage.out ./pkg/parser/ ./pkg/exporter/ && go tool cover -func=coverage.out` and verify that new functions (`ConvertMrkdwnToMarkdown`, `RenderDailyDoc`, `SanitizeDirectoryName`, `WriteMarkdownFile`, `ExpandAndValidatePath`) each have >=80% coverage
- [x] 8.5 Verify constitution alignment: Go-only implementation (Principle II: Go-First Architecture), no new external dependencies (Principle II), `MarkdownWriter` testable without filesystem or services (Development Workflow: Test-Driven), non-fatal errors preserve resilience (Principle V: Concurrency & Resilience), file permissions follow Security First (Principle VI)

<!-- spec-review: passed -->
<!-- code-review: passed -->
