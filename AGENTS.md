# get-out Development Guidelines

Auto-generated from all feature plans. Last updated: 2026-03-11

## Active Technologies
- Go 1.25.0 + Chromedp (CDP), cobra v1.10.2 (CLI), charmbracelet/huh (interactive prompts), charmbracelet/lipgloss (styled output), Google Drive API v3, Google Docs API v1, golang.org/x/oauth2, github.com/zalando/go-keyring v0.2.6 (OS keychain)
- JSON files in `~/.get-out/` (config, token, export index) — no database; secrets optionally stored in OS keychain
- Go 1.25 (existing; no change) + `github.com/unbound-force/gaze/cmd/gaze@latest` (external tool, installed via `go install`); `opencode-ai` npm package (external tool, installed via `npm install -g`) (004-gaze-ci-opencode)
- N/A — no persistent storage; `coverage.out` is an ephemeral workspace file (004-gaze-ci-opencode)

## Project Structure

```text
get-out/
├── cmd/get-out/main.go       # CLI entry point
├── internal/cli/             # Command implementations
│   ├── root.go               # Base command and global flags
│   ├── auth.go               # Google OAuth commands (auth login, auth status)
│   ├── selfservice.go        # Self-service commands (init, doctor, setup-browser)
│   ├── helpers.go            # Shared formatting helpers
│   ├── list.go               # List conversations command
│   ├── export.go             # Export command
│   ├── discover.go           # Discover Slack conversations command
│   └── status.go             # Show export status command
├── pkg/
│   ├── chrome/               # Chrome DevTools Protocol client
│   ├── slackapi/             # Slack API client (browser + bot modes)
│   ├── gdrive/               # Google Drive/Docs API client
│   ├── exporter/             # Export orchestration and indexing
│   ├── parser/               # Slack mrkdwn conversion
│   ├── config/               # Configuration loading
│   ├── secrets/              # SecretStore interface + KeychainStore/FileStore backends
│   └── models/               # Shared domain types (ConversationType, ExportMode)
├── config/                   # Configuration files (gitignored except examples)
│   ├── settings.json         # Application settings (Slack workspace URL, credentials paths, folder ID, etc.)
│   ├── conversations.json    # Conversations to export
│   ├── people.json           # User ID to name mappings
│   └── credentials.json      # Google OAuth credentials
├── specs/                    # Feature specifications
└── ~/.get-out/chrome-data/   # Dedicated Chrome profile (created by setup-browser)
```

## Commands

### Build
```bash
go build -o get-out ./cmd/get-out
```

### Run
```bash
# Initialize config directory (~/.get-out/)
./get-out init

# Authenticate with Google
./get-out auth login

# Check authentication status
./get-out auth status

# Run health checks
./get-out doctor

# Launch Chrome and verify Slack setup (5-step wizard)
# Auto-launches Chrome with dedicated profile at ~/.get-out/chrome-data/
./get-out setup-browser

# List configured conversations
./get-out list --config ./config

# Discover and save conversations from Slack API
./get-out discover --config ./config

# Show export status from checkpoint index
./get-out status --config ./config

# Export (dry run)
./get-out export --dry-run --config ./config

# Export to specific folder ID
./get-out export --folder-id <google-drive-folder-id> --config ./config
```

### Test
```bash
go test -race -count=1 ./...
```

## Code Style

- Go 1.24: Follow standard Go conventions (gofmt, golint)
- Error handling: Wrap errors with context using fmt.Errorf
- Logging: Use progress callbacks for user-facing output

## Documentation Requirements

When making changes, always review and update:
1. **README.md** - Usage examples, flags, configuration
2. **AGENTS.md** - Build commands, project structure
3. **Constitution** - Architectural decisions

## Recent Changes
- 004-gaze-ci-opencode: Added Go 1.25 (existing; no change) + `github.com/unbound-force/gaze/cmd/gaze@latest` (external tool, installed via `go install`); `opencode-ai` npm package (external tool, installed via `npm install -g`)
- 003-distribution: Added init, doctor, auth login/status, setup-browser commands; charmbracelet/huh + lipgloss; config dir moved to ~/.get-out/; macOS signing + Homebrew tap release pipeline
- 001-slack-message-export: Core export functionality with browser mode; --folder-id flag

<!-- MANUAL ADDITIONS START -->

# GOVERNANCE.md

## Core Mission (Mission Command)
- **Strategic Architecture:** Engineers shift from manual coding to directing an "infinite supply of junior developers" (AI agents).
- **Outcome Orientation:** Focus on conveying business value and user intent rather than low-level technical sub-tasks.
- **Intent-to-Context:** Treat specs and rules as the medium through which human intent is manifested into code.

## Behavioral Constraints
- **Zero-Waste Mandate:** No orphaned code, unused dependencies, or "Feature Zombie" bloat.
- **Neighborhood Rule:** Changes must be audited for negative impacts on adjacent modules or the wider ecosystem.
- **Intent Drift Detection:** Evaluation must detect when the implementation drifts away from the original human-written "Statement of Intent."
- **Automated Governance:** Primary feedback is provided via automated constraints, reserving human energy for high-level security and logic.

### Gatekeeping Value Protection

Agents MUST NOT modify values that serve as quality or
governance gates to make an implementation pass. The
following categories are protected:

1. **Coverage thresholds and CRAP scores** — minimum
   coverage percentages, CRAP score limits, coverage
   ratchets
2. **Severity definitions and auto-fix policies** —
   CRITICAL/HIGH/MEDIUM/LOW boundaries, auto-fix
   eligibility rules
3. **Convention pack rule classifications** —
   MUST/SHOULD/MAY designations on convention pack rules
   (downgrading MUST to SHOULD is prohibited)
4. **CI flags and linter configuration** — `-race`,
   `-count=1`, `govulncheck`, `golangci-lint` rules,
   pinned action SHAs
5. **Agent temperature and tool-access settings** —
   frontmatter `temperature`, `tools.write`, `tools.edit`,
   `tools.bash` restrictions
6. **Constitution MUST rules** — any MUST rule in
   `.specify/memory/constitution.md` or hero constitutions
7. **Review iteration limits and worker concurrency** —
   max review iterations, max concurrent Swarm workers,
   retry limits
8. **Workflow gate markers** — `<!-- spec-review: passed
   -->`, task completion checkboxes used as gates, phase
   checkpoint requirements

**What to do instead**: When an implementation cannot
meet a gate, the agent MUST stop, report which gate is
blocking and why, and let the human decide whether to
adjust the gate or rework the implementation. Modifying
a gate without explicit human authorization is a
constitution violation (CRITICAL severity).

### Workflow Phase Boundaries

Agents MUST NOT cross workflow phase boundaries:

- **Specify/Clarify/Plan/Tasks/Analyze/Checklist** phases:
  spec artifacts ONLY (`specs/NNN-*/` directory). No
  source code, test, agent, command, or config changes.
- **Implement** phase: source code changes allowed,
  guided by spec artifacts.
- **Review** phase: findings and minor fixes only. No new
  features.

A phase boundary violation is treated as a process error.
The agent MUST stop and report the violation rather than
proceeding with out-of-phase changes.

### CI Parity Gate

Before marking any implementation task complete or
declaring a PR ready, agents MUST replicate the CI checks
locally. Read `.github/workflows/` to identify the exact
commands CI runs, then execute those same commands. Any
failure is a blocking error — a task is not complete
until all CI-equivalent checks pass locally. Do not rely
on a memorized list of commands; always derive them from
the workflow files, which are the source of truth.

## Technical Guardrails
- **WORM Persistence:** Use Write-Once-Read-Many patterns where data integrity is paramount.

## Council Governance Protocol
- **The Architect:** Must verify that "Intent Driving Implementation" is maintained.
- **The Adversary:** Acts as the primary "Automated Governance" gate for security.
- **The Guardian:** Detects "Intent Drift" to ensure the business value remains intact.

**Rule:** A Pull Request is only "Ready for Human" once all three commands return an **APPROVE** status.

### Review Council as PR Prerequisite

Before submitting a pull request, agents **must** run
`/review-council` and resolve all REQUEST CHANGES
findings until all reviewers return APPROVE. There must
be **minimal to no code changes** between the council's
APPROVE verdict and the PR submission — the council
reviews the final code, not a draft that changes
afterward.

Workflow:

1. Complete all implementation tasks
2. Run CI checks locally (build, test, vet)
3. Run `/review-council` — fix any findings, re-run
   until APPROVE
4. Commit, push, and submit PR immediately after council
   APPROVE
5. Do NOT make further code changes between APPROVE and
   PR submission

Exempt from council review:

- Constitution amendments (governance documents, not code)
- Documentation-only changes (README, AGENTS.md, spec
  artifacts)
- Emergency hotfixes (must be retroactively reviewed)

## Spec-First Development

All changes that modify production code, test code, agent
prompts, embedded assets, or CI configuration **must** be
preceded by a spec workflow. The constitution
(`.specify/memory/constitution.md`) is the highest-
authority document in this project — all work must align
with it.

Two spec workflows are available:

| Workflow | Location | Best For |
|----------|----------|----------|
| **Speckit** | `specs/NNN-name/` | Numbered feature specs with the full pipeline |
| **OpenSpec** | `openspec/changes/name/` | Targeted changes with lightweight artifacts |

**What requires a spec** (no exceptions without explicit
user override):

- New features or capabilities
- Refactoring that changes function signatures, extracts
  helpers, or moves code between packages
- Test additions or assertion strengthening across
  multiple functions
- Agent prompt changes
- CI workflow modifications
- Data model changes (new struct fields, schema updates)

**What is exempt** (may be done directly):

- Constitution amendments (governed by the constitution's
  own Governance section)
- Typo corrections, comment-only changes, single-line
  formatting fixes
- Emergency hotfixes for critical production bugs (must
  be retroactively documented)

When an agent is unsure whether a change is trivial, it
**must** ask the user rather than proceeding without a
spec. The cost of an unnecessary spec is minutes; the
cost of an unplanned change is rework, drift, and broken
CI.

### Website Documentation Gate

When a change affects user-facing behavior, hero
capabilities, CLI commands, or workflows, a GitHub issue
**MUST** be created in the `unbound-force/website`
repository to track required documentation or website
updates. The issue must be created before the
implementing PR is merged.

```bash
gh issue create --repo unbound-force/website \
  --title "docs: <brief description of what changed>" \
  --body "<what changed, why it matters, which pages
          need updating>"
```

**Exempt changes** (no website issue needed):
- Internal refactoring with no user-facing behavior
  change
- Test-only changes
- CI/CD pipeline changes
- Spec artifacts (specs are internal planning documents)

**Examples requiring a website issue**:
- New CLI command or flag added
- Hero capabilities changed (new agent, removed feature)
- Installation steps changed (`uf setup` flow)
- New convention pack added
- Breaking changes to any user-facing workflow

## Knowledge Retrieval

Agents SHOULD prefer Dewey MCP tools over grep/glob/read
for cross-repo context, design decisions, and
architectural patterns. Dewey provides semantic search
across all indexed Markdown files, specs, and web
documentation — returning ranked results with provenance
metadata that grep cannot match.

### Tool Selection Matrix

| Query Intent | Dewey Tool | When to Use |
|-------------|-----------|-------------|
| Conceptual understanding | `dewey_semantic_search` | "How does X work?" |
| Keyword lookup | `dewey_search` | Known terms, FR numbers |
| Read specific page | `dewey_get_page` | Known document path |
| Relationship discovery | `dewey_find_connections` | "How are X and Y related?" |
| Similar documents | `dewey_similar` | "Find specs like this one" |
| Tag-based discovery | `dewey_find_by_tag` | "All pages tagged #decision" |
| Property queries | `dewey_query_properties` | "All specs with status: draft" |
| Filtered semantic | `dewey_semantic_search_filtered` | Semantic search within source type |
| Graph navigation | `dewey_traverse` | Dependency chain walking |

### When to Fall Back to grep/glob/read

Use direct file operations instead of Dewey when:
- **Dewey is unavailable** — MCP tools return errors or
  are not configured
- **Exact string matching is needed** — searching for a
  specific error message, variable name, or code pattern
- **Specific file path is known** — reading a file you
  already know the path to (use Read directly)
- **Binary/non-Markdown content** — Dewey indexes
  Markdown; use grep for Go source, JSON, YAML, etc.

### Graceful Degradation (3-Tier Pattern)

**Tier 3 (Full Dewey)** — semantic + structured search:
- `dewey_semantic_search` — natural language queries
- `dewey_search` — keyword queries
- `dewey_get_page`, `dewey_find_connections`,
  `dewey_traverse` — structured navigation
- `dewey_find_by_tag`, `dewey_query_properties` —
  metadata queries

**Tier 2 (Graph-only, no embedding model)** — structured
search only:
- `dewey_search` — keyword queries (no embeddings needed)
- `dewey_get_page`, `dewey_traverse`,
  `dewey_find_connections` — graph navigation
- `dewey_find_by_tag`, `dewey_query_properties` —
  metadata queries
- Semantic search unavailable — use exact keyword matches

**Tier 1 (No Dewey)** — direct file access:
- Use Read tool for direct file access
- Use Grep for keyword search across the codebase
- Use Glob for file pattern matching

<!-- MANUAL ADDITIONS END -->
