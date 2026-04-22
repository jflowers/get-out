# Quickstart: Sensitivity Filter for Local Markdown Export

**Feature**: `005-sensitivity-filter` | **Date**: 2026-04-21

## Prerequisites

1. **get-out** is installed and configured (`get-out init` completed)
2. **Local markdown export** is configured (`localExportOutputDir` in settings.json or `--local-export-dir` flag)
3. At least one conversation has `"localExport": true` in `conversations.json`

## Setup

### 1. Install Ollama

```bash
# macOS (Homebrew)
brew install ollama

# Or download from https://ollama.com/download
```

### 2. Start the Ollama Server

```bash
ollama serve
```

Ollama runs on `http://localhost:11434` by default. Leave this running in a terminal or configure it as a background service.

### 3. Pull the Granite Guardian Model

```bash
ollama pull granite-guardian:8b
```

This downloads the ~4.7GB model. It only needs to be done once.

### 4. Configure Sensitivity Filtering

Add the `ollama` section to your `~/.get-out/settings.json`:

```json
{
  "slackWorkspaceUrl": "https://app.slack.com",
  "localExportOutputDir": "~/.get-out/export",
  "ollama": {
    "enabled": true,
    "endpoint": "http://localhost:11434",
    "model": "granite-guardian:8b"
  }
}
```

**Fields**:
| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `enabled` | Yes | `false` | Must be `true` to activate filtering |
| `endpoint` | No | `http://localhost:11434` | Ollama REST API URL |
| `model` | No | `granite-guardian:8b` | Model to use for classification |

## Verification

### Run Doctor

```bash
get-out doctor
```

Expected output (when everything is configured correctly):

```
─────────────────────────────────────────
  get-out doctor
─────────────────────────────────────────

  ✓ Config directory exists (secret storage: keychain)
  ✓ credentials present
  ✓ token present
  ✓ OAuth token is valid
  ✓ Drive API: connected as user@example.com
  ✓ conversations.json: 5 conversation(s) configured
  ✓ people.json exists
  ✓ Chrome reachable on port 9222
  ✓ Slack tab found (1 tab(s))
  ✓ export-index.json is healthy
  ✓ Ollama: OK (http://localhost:11434)
  ✓ Sensitivity model: OK (granite-guardian:8b)

─────────────────────────────────────────
  12 passed · 0 warnings · 0 failures
─────────────────────────────────────────
```

### Common Doctor Failures

**Ollama not running**:
```
  ✗ Ollama: FAIL — not reachable at http://localhost:11434
    → Start Ollama: ollama serve
```

**Model not pulled**:
```
  ✓ Ollama: OK (http://localhost:11434)
  ✗ Sensitivity model: FAIL — "granite-guardian:8b" not found
    → Pull the model: ollama pull granite-guardian:8b
```

## Usage

### Standard Export with Sensitivity Filtering

```bash
# Export with filtering active (uses settings.json config)
get-out export --local-export-dir ~/.get-out/export
```

Messages classified as sensitive are excluded from markdown files. Google Docs exports are unaffected and include all messages.

### Bypass Filtering for a Single Run

```bash
# Skip sensitivity filtering for this run only
get-out export --no-sensitivity-filter --local-export-dir ~/.get-out/export
```

This exports all messages to markdown without any Ollama calls. Your settings.json configuration is unchanged — the next run without the flag will use filtering normally.

### Override Ollama Endpoint

```bash
# Use a different Ollama instance for this run
get-out export --ollama-endpoint http://192.168.1.100:11434
```

### Dry Run

```bash
# See what would be exported (no actual export or classification)
get-out export --dry-run --local-export-dir ~/.get-out/export
```

## Output

### Markdown File with Sensitivity Metadata

When filtering is active, each markdown file includes a `sensitivity` section in its YAML frontmatter:

```yaml
---
conversation: HR Team
type: channel
date: "2026-04-21"
participants:
  - Alice Johnson
  - Bob Smith
  - Carol Davis
sensitivity:
  filtered_count: 3
  categories:
    hr: 2
    financial: 1
---

**10:15 AM -- Alice Johnson**

Team standup notes for today: Sprint review at 2pm, retrospective at 3pm.

**10:32 AM -- Bob Smith**

Updated the PTO policy document — please review when you get a chance.

**11:45 AM -- Carol Davis**

Reminder: benefits enrollment deadline is Friday.
```

In this example, 3 messages were filtered out (2 HR-related, 1 financial). The remaining messages are general/non-sensitive.

### When All Messages Are Filtered

If every message in a daily doc is classified as sensitive, no markdown file is produced for that date. This is by design (FR-010) — an empty file would be misleading.

## Error Scenarios

### Ollama Unavailable During Export

```
Error: Ollama is not reachable at http://localhost:11434

Sensitivity filtering is enabled but the Ollama server is not running.

To fix:
  • Start Ollama:  ollama serve
  • Or bypass for this run:  get-out export --no-sensitivity-filter
```

The export fails immediately — no files are written. This is intentional (FR-007): the system never silently falls back to unfiltered export.

### Model Not Available

```
Error: Model "granite-guardian:8b" is not available

The sensitivity filter requires this model but it is not installed.

To fix:
  • Pull the model:  ollama pull granite-guardian:8b
  • Or bypass for this run:  get-out export --no-sensitivity-filter
```

### Ollama Fails Mid-Export

If Ollama becomes unreachable after some conversations have been exported:

```
Error: sensitivity classification failed for "HR Team" (2026-04-21): 
  connection refused at http://localhost:11434

Successfully exported before failure:
  - Engineering (12 messages, 3 docs)
  - Design Team (8 messages, 2 docs)

To resume: fix Ollama and run: get-out export --sync
```

## FAQ

**Q: Does sensitivity filtering affect Google Docs exports?**  
A: No. Google Docs exports always include all messages regardless of sensitivity filtering settings (FR-006).

**Q: What happens to existing markdown files when I enable filtering?**  
A: Nothing. Previously exported files are not retroactively re-filtered (FR-012). Only new exports are filtered.

**Q: What if the classifier makes a mistake?**  
A: The system errs on the side of caution — false positives (normal messages excluded) are preferred over false negatives (sensitive messages included). If you need all messages, use `--no-sensitivity-filter`.

**Q: Can I use a different model?**  
A: Yes. Set `"model": "your-model-name"` in the `ollama` section of settings.json. The model must support the `/api/generate` endpoint and produce structured JSON output.

**Q: Does this work with remote Ollama instances?**  
A: Yes. Set the `endpoint` in settings.json or use `--ollama-endpoint` to point to a remote Ollama server.
