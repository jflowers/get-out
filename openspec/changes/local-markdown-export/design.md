## Context

get-out currently exports Slack conversations exclusively to Google Docs via the `DocWriter` → `gdrive.Client` pipeline. The parser (`ConvertMrkdwnWithLinks`) produces plain text with link annotations, which `DocWriter.messageToBlock()` converts to `gdrive.MessageBlock` structures before calling `BatchAppendMessages`. There is no local filesystem output.

The `Settings` struct already contains a `LocalExportOutputDir` field (marked "for future use"), and the export index checkpoint system tracks per-date, per-conversation export state -- both of which this feature activates.

This design adds a parallel markdown output path that reuses the same parsed message data but formats it as standard markdown files with YAML frontmatter, suitable for Dewey disk source indexing.

## Goals / Non-Goals

### Goals
- Write local markdown files during export, one per conversation per date, mirroring the daily-doc granularity of Google Docs export
- Produce markdown with YAML frontmatter containing conversation metadata (name, type, date, participants) for Dewey indexing
- Preserve Slack formatting (bold, italic, strikethrough, code, links) in markdown output
- Support incremental export -- only write markdown for dates not yet written locally
- Make local export opt-in at two levels: globally via `localExportOutputDir` settings field / `--local-export-dir` CLI flag, and per-conversation via `localExport: true` in `conversations.json`
- Ensure local markdown export failures do not block the primary Google Docs export

### Non-Goals
- Downloading images locally (images will be referenced by URL or omitted)
- Bidirectional sync (markdown files are write-once, not updated when Google Docs change)
- Exporting thread replies to separate markdown files (threads are included inline in their parent conversation's daily file for simpler Dewey indexing)
- Configuring Dewey sources automatically (get-out writes files; Dewey configuration is the user's responsibility per Composability First)
- Real-time or watch-mode export

## Decisions

### D1: New `MarkdownWriter` component in `pkg/exporter/`

Create `pkg/exporter/mdwriter.go` with a `MarkdownWriter` struct that accepts `[]slackapi.Message` and resolver context, producing a complete markdown file as `[]byte`. This mirrors the `DocWriter` pattern but targets the filesystem instead of the Google Docs API.

**Rationale**: Keeps the markdown formatting logic isolated and testable without any filesystem or Google API dependencies (Testability principle). The writer returns bytes; the caller decides where to write them.

### D2: Hook into the daily-doc export loop, not a separate pass

Markdown files are written inside the same `for _, date := range dates` loop in `ExportConversation()`, inside the daily-doc iteration, immediately after the `docWriter.WriteMessages()` call and checkpoint save. This ensures markdown output tracks exactly what was exported to Drive.

Before writing, the markdown writer checks if the target file already exists on disk (file-existence check, independent of the checkpoint index). If the file exists, the write is skipped. This means deleted markdown files can be regenerated on a full re-export, unlike Google Docs which are governed by the checkpoint index.

**Rationale**: A separate pass would require re-fetching messages or caching them in memory. The inline approach reuses the already-fetched `msgs` slice with zero additional API calls. The file-existence skip check (rather than checkpoint-based) gives users a simple recovery path: delete the file and re-export.

### D3: Parser mode for markdown-preserving conversion

Add a new function `ConvertMrkdwnToMarkdown` in `pkg/parser/mrkdwn.go` that preserves formatting markers instead of stripping them. The existing `ConvertMrkdwnWithLinks` strips `*bold*` → `bold` for Google Docs (which applies formatting via API). Markdown output needs `*bold*` → `**bold**` (markdown bold), `_italic_` → `*italic*` (markdown italic), `` `code` `` preserved, and ` ```blocks``` ` converted to fenced code blocks.

**Rationale**: A second conversion function is simpler and more testable than adding a mode parameter to the existing function. The existing function's behavior is unchanged, eliminating regression risk.

### D4: Activate the existing `LocalExportOutputDir` settings field

The `Settings` struct already has `LocalExportOutputDir string json:"localExportOutputDir,omitempty"` at `pkg/config/types.go:18`. This field becomes the trigger: when non-empty, local markdown export is enabled. The `--local-export-dir` CLI flag overrides it.

**Rationale**: Reuses existing infrastructure. No config schema migration needed. Consistent with how `folder_id` / `--folder-id` works for Google Drive.

### D5: Flat directory structure with conversation-name subdirectories

```
~/.get-out/export/
  dm-john-smith/
    2026-01-15.md
    2026-01-16.md
  channel-general/
    2026-02-01.md
  group-team-chat/
    2026-02-01.md
```

Directory names use `{type-prefix}-{name}` in kebab-case (sanitized for filesystem safety). The type prefix is mapped from the canonical `ConversationType` value:

| `ConversationType` | Directory Prefix |
|---|---|
| `dm` | `dm` |
| `mpim` | `group` |
| `channel` | `channel` |
| `private_channel` | `private-channel` |

This differs from the Google Drive structure which uses `DM - John Smith` folder names. Note: the YAML frontmatter `type` field uses the canonical model value (e.g., `private_channel`), while the directory prefix uses the mapped label (e.g., `private-channel`).

**Rationale**: Filesystem-safe naming avoids spaces and special characters. The `{type}-` prefix groups conversations by type in directory listings. Kebab-case is the convention for Dewey disk sources. The `mpim` → `group` mapping provides a human-friendly label while preserving the canonical type in frontmatter metadata.

### D6: Markdown write failures are non-fatal

If writing a markdown file fails (disk full, permission error), the error is logged via the progress callback but does not abort the export. The Google Docs export continues unaffected.

**Rationale**: The primary export target is Google Docs. Local markdown is supplementary. Users should not lose their Drive export because of a local disk issue. This aligns with the Resilience principle (Constitution V).

### D7: Atomic file writes with cleanup

Markdown files are written atomically: write to a temp file (`.tmp-` prefixed) in the same directory, then `os.Rename` to the final path. The `.tmp-` prefix ensures Dewey does not index incomplete files (Dewey indexes `*.md` files, not dotfiles). If the write fails, the temp file is removed (best-effort cleanup).

**Rationale**: Dewey may be indexing the export directory concurrently. A partially-written file could produce corrupt search results. The cleanup-on-failure prevents orphaned temp files from accumulating.

### D8: Thread messages included inline as blockquotes

Thread replies are included inline within the parent conversation's daily markdown file. After the parent message content, a "**Thread replies:**" marker line is added, followed by each reply rendered as a blockquoted message with bold timestamp and sender. They are not exported to separate thread subdirectories as they are in Google Drive.

**Rationale**: For Dewey semantic search, thread context is most useful when co-located with the conversation flow. Separate thread files would fragment search results and complicate the file structure for a feature whose primary consumer is an AI search index. Blockquote formatting visually distinguishes thread replies from top-level messages while keeping them in the same search chunk.

### D9: Per-conversation `localExport` opt-in (default false)

Add a `LocalExport bool` field to `ConversationConfig` (JSON key `"localExport"`). Local markdown is written for a conversation only when both conditions are true:
1. `localExportDir` is configured (global enablement)
2. `conv.LocalExport == true` (per-conversation opt-in)

The field defaults to `false` when omitted from `conversations.json`, meaning no conversations produce local markdown unless explicitly marked. This is independent of the `Export` field -- a conversation can be exported to Google Docs without producing local markdown, or (in principle) the reverse, though the current export loop requires `Export: true` to fetch messages.

**Rationale**: Not all conversations contain content worth indexing for AI agents. DMs with casual messages or sensitive HR discussions should not end up in a Dewey-indexed directory by default. Opt-in per conversation gives users granular control over what enters their local knowledge base, consistent with how the existing `export` field requires explicit opt-in for Google Docs export. The `false` default ensures zero surprise disk writes.

## Risks / Trade-offs

### R1: Formatting divergence between markdown and Google Docs

The markdown output will not be identical to the Google Docs output. Google Docs supports rich formatting (fonts, colors, inline images) that markdown cannot represent. The markdown output prioritizes semantic content over visual fidelity.

**Mitigation**: Document the differences. Markdown output is for machine consumption (Dewey), not human reading. Content accuracy matters more than formatting accuracy.

### R2: Disk usage for large workspaces

A workspace with many channels and years of history could produce significant local file volume. Markdown files are compact (typically 1-10 KB per daily file), but thousands of conversations over years could accumulate.

**Mitigation**: Feature is opt-in. Users explicitly configure the export directory. No default auto-export behavior.

### R3: Image references may be ephemeral

Slack image URLs expire. Markdown files will contain `![image](slack-url)` references that may become broken over time. The Google Docs export downloads and re-uploads images to Drive, which is durable.

**Mitigation**: Document this limitation. A future enhancement could download images locally, but that is a non-goal for this change.

### R4: Incremental export has two independent skip mechanisms

The markdown skip logic uses file-existence checks (independent of the checkpoint index), but the export loop itself skips dates via the checkpoint index in `--sync` mode. This means:
- File-existence skip: if the markdown file exists, skip writing. If deleted, it can be regenerated.
- Checkpoint skip: if `--sync` mode skips a date entirely (no messages fetched), the markdown writer is never invoked for that date.

A deleted markdown file will only be regenerated if the export loop re-processes that date, which requires running without `--sync` (full re-export) or with `--from`/`--to` flags covering that date.

**Mitigation**: Users can re-run without `--sync` to regenerate deleted markdown files. The file-existence check prevents unnecessary rewrites during normal operation. The two mechanisms are clearly separated: the export loop controls which dates are processed, the markdown writer controls which files are written.
