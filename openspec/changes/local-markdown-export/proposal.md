## Why

get-out exports Slack conversations to Google Docs on Drive. These conversations contain team decisions, design discussions, and project context -- but they are only accessible in Google Docs. AI agents using Dewey cannot search Slack conversation history because the exported content never reaches the local filesystem.

By writing a local markdown copy during export, Dewey can index conversation history as a disk source, enabling semantic search across Slack conversations alongside specs, code, and other knowledge artifacts. This follows the zero-coupling composability pattern: get-out writes files, Dewey reads files, no API dependency between them.

## What Changes

Add an optional local markdown export path alongside the existing Google Docs export. Local markdown export is controlled at two levels: a global `localExportOutputDir` setting (or `--local-export-dir` CLI flag) enables the feature, and a per-conversation `localExport: true` field in `conversations.json` opts individual conversations in. During the per-daily-doc export loop, after writing to Google Docs, also write a markdown file to the configured local directory for opted-in conversations. The markdown output uses YAML frontmatter for metadata (conversation name, type, date, participants) and standard markdown formatting for message content.

## Capabilities

### New Capabilities
- `local-markdown-export`: Write markdown copies of opted-in conversations to `~/.get-out/export/` (or a configurable path), organized by conversation name and date
- `--local-export-dir` flag: CLI flag to specify or override the local export directory
- `localExportDir` config field: Settings field to enable and configure the local export directory persistently
- `localExport` conversation field: Per-conversation opt-in boolean in `conversations.json` (default `false`) controlling which conversations produce local markdown
- `markdown-writer`: New internal component (`pkg/exporter/mdwriter.go`) that converts Slack messages to markdown with YAML frontmatter

### Modified Capabilities
- `export` command: Gains awareness of local markdown export; writes markdown for conversations with `localExport: true` when a local export directory is configured
- `Settings` struct: Activates existing `LocalExportOutputDir` field
- `ConversationConfig` struct: Extended with `LocalExport` boolean field

### Removed Capabilities
- None

## Impact

- **pkg/exporter/**: New `mdwriter.go` file; modifications to the export loop in `exporter.go` to call the markdown writer after each daily doc
- **pkg/config/**: `ConversationConfig` gains `LocalExport` field; `Settings` activates existing `LocalExportOutputDir` field
- **internal/cli/export.go**: New `--local-export-dir` flag wired to settings
- **Filesystem**: Creates `~/.get-out/export/<conversation-name>/<date>.md` files when enabled
- **No changes** to: gdrive, slackapi, chrome, secrets, models, or the checkpoint index
- **Parser**: New `ConvertMrkdwnToMarkdown` function added (existing functions unchanged)

## Constitution Alignment

Assessed against the get-out constitution.

### I. Session-Driven Extraction

**Assessment**: N/A

This change does not affect data extraction mechanics. It operates on messages already fetched via the existing session-driven extraction pipeline.

### II. Go-First Architecture

**Assessment**: PASS

The markdown writer is implemented entirely in Go using standard library (`os`, `fmt`, `text/template` or string building). No external dependencies are added. The single-binary deployment model is preserved.

### III. Stealth & Reliability

**Assessment**: N/A

Local file writing does not interact with Slack's browser session or API. No risk of detection or rate limiting.

### IV. Two-Tier Extraction Strategy

**Assessment**: N/A

This change adds a new output format, not a new extraction method. Both Tier 1 (API) and Tier 2 (DOM) extraction remain unchanged.

### V. Concurrency & Resilience

**Assessment**: PASS

Markdown writing integrates with the existing checkpoint system. Files are written atomically (write to temp, rename) to prevent partial files. The incremental export model means only new messages produce new markdown files. Failed markdown writes do not block the primary Google Docs export.

### VI. Security First

**Assessment**: PASS

Markdown files contain only message content -- no tokens, credentials, or session data. Files are written with appropriate permissions (0644 for files, 0755 for directories) to the user's home directory.

### VII. Output Format

**Assessment**: PASS

The existing Google Docs output requirement is preserved unchanged. Local markdown is an additional, optional output format. Slack mrkdwn formatting is preserved in markdown output (bold, italic, links, code blocks) since markdown is a natural fit for the source format.

### VIII. Google Drive Integration

**Assessment**: N/A

No changes to Google Drive integration. The local markdown export is independent of Drive operations.

### IX. Documentation Maintenance

**Assessment**: PASS

Implementation must update README.md with the new `--local-export-dir` flag, AGENTS.md with the new file in the project structure, and config examples with the new `localExportDir` field.
