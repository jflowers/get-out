## ADDED Requirements

_None — this is a structural refactoring with no behavioral changes._

## MODIFIED Requirements

### Requirement: runDiscover Complexity

Previously: `runDiscover` was a single function with cyclomatic complexity 44, containing member fetching, user profile fetching, merging, and file writing logic.

Now: `runDiscover` is a thin orchestrator (cx ≤ 15) that calls `collectChannelMembers`, `fetchUserProfiles`, and `writePeopleJSON`. All extracted functions are testable independently.

#### Scenario: Discover produces identical output
- **GIVEN** the same conversations.json, Chrome session, and Slack workspace
- **WHEN** `get-out discover` is run before and after the decomposition
- **THEN** the output to stdout and the resulting people.json MUST be identical

### Requirement: runExport Complexity

Previously: `runExport` was a single function with cyclomatic complexity 37, containing settings resolution, config loading, date parsing, prerequisite checking, and export orchestration.

Now: `runExport` is a thin orchestrator (cx ≤ 15) that calls `resolveExportFolderID`, `loadExportDependencies`, and `parseDateRange`. All extracted functions are testable independently.

#### Scenario: Export produces identical output
- **GIVEN** the same configuration, Chrome session, and Slack workspace
- **WHEN** `get-out export` is run before and after the decomposition
- **THEN** the export behavior, folder structure, and summary output MUST be identical

## REMOVED Requirements

_None._
