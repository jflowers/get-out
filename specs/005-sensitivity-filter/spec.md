# Feature Specification: Sensitivity Filter for Local Markdown Export

**Feature Branch**: `005-sensitivity-filter`
**Created**: 2026-04-21
**Status**: Draft
**Input**: User description: "Sensitivity filter for local markdown export"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Sensitive Messages Excluded from Markdown Export (Priority: P1)

A user exports Slack conversations to local markdown files for Dewey semantic search indexing. Some messages contain sensitive content -- HR discussions about specific individuals, salary details, or legal matters. When sensitivity filtering is enabled, the system classifies each message before writing it to the markdown file and excludes any message determined to be sensitive. The Google Docs export is unaffected and continues to include all messages.

**Why this priority**: This is the core value of the feature. Without per-message classification and exclusion, sensitive content enters the Dewey search index where AI agents can surface it in unrelated queries. This is the minimum viable protection.

**Independent Test**: Can be fully tested by exporting a conversation containing a mix of normal and sensitive messages with sensitivity filtering enabled, then verifying that the resulting markdown file contains only the non-sensitive messages while the Google Docs export contains all messages.

**Acceptance Scenarios**:

1. **Given** sensitivity filtering is enabled and Ollama is running with the configured model, **When** the user exports a conversation containing both normal and sensitive messages to local markdown, **Then** the markdown file contains only non-sensitive messages and the Google Docs export contains all messages.
2. **Given** sensitivity filtering is enabled, **When** a daily doc contains 10 messages where 3 are classified as sensitive, **Then** the markdown file contains 7 messages and the YAML frontmatter includes a `sensitivity` section showing `filtered_count: 3` with a per-category breakdown whose values sum to `filtered_count`.
3. **Given** sensitivity filtering is enabled, **When** all messages in a daily doc are classified as sensitive, **Then** no markdown file is written for that date (empty file is not produced).

---

### User Story 2 - Export Fails When Ollama Is Unavailable (Priority: P1)

A user has sensitivity filtering configured in their settings. They run an export, but Ollama is not running or is unreachable. The export fails immediately with a clear error message explaining the problem and how to fix it, rather than silently exporting unfiltered content.

**Why this priority**: Equal priority with filtering itself. Silent fallback to unfiltered export defeats the purpose of the feature. Users who enable sensitivity filtering have made an explicit security choice that must be honored.

**Independent Test**: Can be fully tested by configuring sensitivity filtering with an unreachable Ollama endpoint, running the export, and verifying the export fails with an actionable error message before any files are written.

**Acceptance Scenarios**:

1. **Given** sensitivity filtering is enabled in settings and Ollama is not running, **When** the user runs `get-out export`, **Then** the export fails with an error message that identifies the unreachable endpoint and suggests either starting Ollama or disabling filtering with `--no-sensitivity-filter`.
2. **Given** sensitivity filtering is enabled and Ollama is running but the configured model is not available, **When** the user runs `get-out export`, **Then** the export fails with an error message identifying the missing model and suggesting how to pull it.
3. **Given** sensitivity filtering is enabled and Ollama becomes unreachable mid-export (after some conversations have been processed), **When** the next conversation is attempted, **Then** the export stops with an error and reports which conversations were successfully exported before the failure.

---

### User Story 3 - Opt-Out of Sensitivity Filtering for a Single Run (Priority: P2)

A user has sensitivity filtering configured in settings but needs to run a quick export without it -- perhaps Ollama is temporarily down for maintenance or they are exporting a known-safe conversation. They use a command-line flag to disable filtering for this run only, without changing their persistent settings.

**Why this priority**: Provides an escape hatch when the hard gate would block legitimate work. Less critical than the core filtering and failure behaviors, but essential for practical day-to-day use.

**Independent Test**: Can be fully tested by configuring sensitivity filtering, running the export with `--no-sensitivity-filter`, and verifying that all messages appear in the markdown output without any Ollama calls.

**Acceptance Scenarios**:

1. **Given** sensitivity filtering is enabled in settings, **When** the user runs `get-out export --no-sensitivity-filter`, **Then** all messages are exported to markdown without sensitivity classification, and no Ollama calls are made.
2. **Given** sensitivity filtering is enabled in settings, **When** the user runs `get-out export --no-sensitivity-filter`, **Then** the subsequent export (without the flag) resumes using sensitivity filtering normally.

---

### User Story 4 - Health Checks for Ollama and Model (Priority: P3)

A user runs `get-out doctor` to verify their setup. When sensitivity filtering is configured, the doctor command checks that Ollama is reachable and the required model is available, reporting clear pass/fail status for each check.

**Why this priority**: Supports setup and troubleshooting. Not required for the core filtering workflow, but prevents confusing failures during export by catching configuration issues early.

**Independent Test**: Can be fully tested by running `get-out doctor` with various Ollama configurations (running/stopped, model present/absent) and verifying the correct health check output for each scenario.

**Acceptance Scenarios**:

1. **Given** sensitivity filtering is configured and Ollama is running with the model available, **When** the user runs `get-out doctor`, **Then** the health checks show "Ollama: OK" and "Sensitivity model: OK".
2. **Given** sensitivity filtering is configured and Ollama is not running, **When** the user runs `get-out doctor`, **Then** the health check shows "Ollama: FAIL" with an actionable message.
3. **Given** sensitivity filtering is not configured, **When** the user runs `get-out doctor`, **Then** Ollama health checks are not displayed (no noise for unconfigured features).

---

### Edge Cases

- What happens when Ollama returns a malformed response (invalid JSON, missing fields)? The system retries once. If the second attempt also fails, all messages in the batch are treated as sensitive (err on the side of caution) and excluded from the markdown output.
- What happens when a conversation has `localExport: true` but sensitivity filtering is not configured? The conversation is exported to markdown without filtering (same behavior as before this feature).
- What happens when a message contains only an image or file attachment with no text? Messages with no text content are treated as non-sensitive (there is no content to classify).
- What happens when a conversation has zero messages for a date after filtering? No markdown file is written for that date.
- What happens when Ollama is slow (response takes over 60 seconds per batch)? The classification times out and the export fails with a timeout error, consistent with the hard-gate posture.
- What happens when the same daily doc is exported again in a subsequent sync run? The file-existence skip check applies -- if the markdown file already exists, it is not re-processed. Previously exported files are not retroactively re-filtered.
- What happens when `--dry-run` is combined with sensitivity filtering? Sensitivity classification is skipped (no Ollama calls are made). The dry-run output does not reflect what would be filtered.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST classify each message for sensitivity before including it in the local markdown output. Messages classified as sensitive MUST be excluded from the markdown file.
- **FR-002**: Sensitivity classification MUST use a locally-running Ollama instance with a configurable model. No message content is sent to cloud services for classification.
- **FR-003**: Classification MUST operate in batch mode -- all new messages for a single daily doc are sent as one classification request. Each message receives an individual sensitive/not-sensitive determination within the batch response.
- **FR-004**: The classification MUST categorize sensitive content into one of: hr, legal, financial, health, termination, or none.
- **FR-005**: The markdown file YAML frontmatter MUST include a `sensitivity` section when filtering is active, containing `filtered_count` and a per-category breakdown of filtered messages.
- **FR-006**: Sensitivity filtering MUST apply only to the local markdown export path. The Google Docs export MUST continue to include all messages regardless of sensitivity classification.
- **FR-007**: When sensitivity filtering is configured and Ollama is unreachable or the model is unavailable, the export MUST fail with a clear, actionable error message. The system MUST NOT silently fall back to unfiltered export.
- **FR-008**: Users MUST be able to bypass sensitivity filtering for a single export run using a `--no-sensitivity-filter` command-line flag without changing their persistent configuration.
- **FR-009**: When classification of a message fails (malformed response after retry), the message MUST be treated as sensitive and excluded from the markdown output (err on the side of caution).
- **FR-010**: When all messages in a daily doc are filtered as sensitive, no markdown file MUST be produced for that date.
- **FR-011**: The `doctor` command MUST include Ollama health checks (reachability and model availability) when sensitivity filtering is configured.
- **FR-012**: Previously exported markdown files MUST NOT be retroactively re-filtered when sensitivity filtering is enabled. The existing file-existence skip logic continues to apply.
- **FR-013**: Messages with no text content (image-only, file-only) MUST be treated as non-sensitive and included in the markdown output.
- **FR-014**: Classification MUST err on the side of caution. When the classifier is uncertain, the message MUST be classified as sensitive.
- **FR-015**: The sensitivity classifier prompt MUST distinguish between content about specific individuals (sensitive) and general policy discussions (not sensitive). For example, "updating the PTO policy" is not sensitive, but "Sarah's excessive absences" is sensitive.

### Key Entities

- **SensitivityResult**: Represents the classification outcome for a single message -- whether it is sensitive, the category of sensitivity (hr, legal, financial, health, termination, none), a confidence score (0.0-1.0), and a brief reasoning explanation.
- **FilterResult**: Aggregates filtering outcomes for a batch of messages in a single daily doc -- the count of filtered messages, the per-category breakdown, and the list of messages that passed filtering.
- **OllamaConfig**: Represents the Ollama connection settings -- endpoint URL, model name, and enabled/disabled state.

### Assumptions

- Users who enable sensitivity filtering have Ollama installed and are comfortable managing a local model. This is a power-user feature, not a default behavior.
- The Granite Guardian model (or equivalent) is capable of classifying Slack-style chat messages, which are shorter and more contextual than the meeting transcripts it was designed for. The prompt will be adapted for this context.
- Classification accuracy does not need to be perfect. The system errs on the side of caution -- false positives (normal messages excluded) are acceptable, while false negatives (sensitive messages included) should be minimized.
- The batch prompt approach (all messages for a daily doc in one request) fits within the model's context window. For typical sync runs with 5-20 new messages per conversation, this is well within limits.
- Ollama runs on localhost by default. Users with remote Ollama instances can configure the endpoint in settings.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Messages classified as sensitive by the configured model are excluded from 100% of local markdown files produced while filtering is active.
- **SC-002**: The export fails with a clear error within 5 seconds when Ollama is unreachable, before any markdown files are written for the current run.
- **SC-003**: Sensitivity filtering adds no more than 5 seconds of overhead per daily doc during a typical sync-mode export (5-20 new messages).
- **SC-004**: The `--no-sensitivity-filter` flag allows a complete export without any Ollama interaction, with zero impact on subsequent filtered runs.
- **SC-005**: When filtering is active, every markdown file produced includes accurate sensitivity metadata in its YAML frontmatter reflecting the count and categories of filtered messages.
- **SC-006**: Zero sensitive message content is transmitted to cloud services for classification. All classification occurs via the locally-running Ollama instance.
