## Why

`runDiscover` (cx=44, CRAP=1980) and `runExport` (cx=37, CRAP=1406) are the two highest-CRAP functions in the codebase. Both are monolithic cobra RunE handlers that mix argument parsing, dependency construction, external service calls, business logic, and output formatting in a single function. Their cyclomatic complexity is so high that even 100% coverage would leave CRAP above threshold (CRAP at 100% = cx).

Earlier sessions extracted some logic (`selectConversations`, `validateExportFlags`, `formatExportDryRun`, `formatExportSummary`, `buildPeopleFromUsers`, `mergePeople`) but the residual handlers remain at cx=44 and cx=37 — well above the decomposition threshold.

This change continues the extract-and-test pattern to bring both functions below the `decompose_and_test` recommendation from gaze's `fix_strategy` field.

## What Changes

Extract additional testable logic from both handlers into pure or near-pure functions. The cobra handlers become thin orchestrators that wire dependencies and call the extracted functions. No behavioral changes — the export and discover commands produce identical output.

### From runDiscover (cx=44):
- **`collectChannelMembers`**: The member-fetching loop that iterates conversations, calls `GetConversationMembers` with pagination, and accumulates unique member IDs
- **`fetchUserProfiles`**: The user-info fetching loop that calls `GetUserInfo` for each member, skipping already-known users
- **`writePeopleJSON`**: The merge + marshal + write-file logic

### From runExport (cx=37):
- **`resolveExportFolderID`**: The settings → flag → default folder ID resolution logic
- **`loadExportDependencies`**: The config loading, people loading, conversation selection, and prerequisite checking block
- **`parseDateRange`**: The date flag parsing + end-of-day adjustment logic

## Capabilities

### New Capabilities
- None — all extracted functions are internal helpers

### Modified Capabilities
- `runDiscover`: Reduced from cx=44 to ~15 by extracting 3 functions
- `runExport`: Reduced from cx=37 to ~12 by extracting 3 functions

### Removed Capabilities
- None

## Impact

- **Files changed**: `internal/cli/discover.go`, `internal/cli/export.go`, new test files for extracted functions
- **No behavioral changes**: All external-facing output, flag handling, and error messages remain identical
- **CRAPload reduction**: Removes 2 entries from CRAPload (the two highest). The extracted functions will have low complexity and can be tested independently.
- **No changes to `pkg/` packages**: All work is in `internal/cli/`

## Constitution Alignment

### I. Session-Driven Extraction

**Assessment**: N/A

No changes to the extraction strategy or Chrome CDP interaction.

### II. Go-First Architecture

**Assessment**: PASS

Pure refactoring of Go code. No new dependencies.

### III. Stealth & Reliability

**Assessment**: N/A

No changes to network behavior or API interactions.

### IV. Testability

**Assessment**: PASS

This change directly improves testability — the extracted functions accept explicit parameters and return values, making them testable without Chrome, Slack, or Google services. The cobra handlers remain untestable (they construct real clients) but they're now thin enough that their CRAP scores drop below threshold.
