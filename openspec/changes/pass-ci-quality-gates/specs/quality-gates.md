## ADDED Requirements

### Requirement: CRAPload Gate Compliance

The project MUST have a CRAPload count of 10 or fewer when measured by `gaze report` with `--crap-threshold=15`. This means no more than 10 functions across all packages MAY have a CRAP score ≥ 15.

#### Scenario: CRAPload passes CI gate
- **GIVEN** all source files in `./internal/...` and `./pkg/...` are compiled and tested with `go test -coverprofile=coverage.out ./...`
- **WHEN** `gaze report ./... --coverprofile=coverage.out --max-crapload=10` is executed
- **THEN** the command MUST exit with code 0 (CRAPload ≤ 10)

#### Scenario: Zero-coverage high-complexity functions are tested
- **GIVEN** a function with 0% line coverage and cyclomatic complexity ≥ 10
- **WHEN** tests are added that exercise the function's primary paths
- **THEN** the function's line coverage MUST reach at least 60%, reducing its CRAP score below the threshold of 15

#### Scenario: High-complexity functions are decomposed
- **GIVEN** a function with cyclomatic complexity > 20
- **WHEN** the function is decomposed into smaller extracted functions
- **THEN** each extracted function MUST have cyclomatic complexity ≤ 10 AND the parent function's complexity MUST decrease by at least 50%

### Requirement: GazeCRAPload Gate Compliance

The project MUST have a GazeCRAPload count of 5 or fewer when measured by `gaze report` with `--gaze-crap-threshold=15`. This means no more than 5 functions MAY have a GazeCRAP score ≥ 15.

#### Scenario: GazeCRAPload passes CI gate
- **GIVEN** all source files are analyzed with `gaze report ./... --max-gaze-crapload=5`
- **WHEN** the command executes
- **THEN** it MUST exit with code 0 (GazeCRAPload ≤ 5)

#### Scenario: Tests assert on contractual side effects
- **GIVEN** a test function that exercises a function returning a value or error
- **WHEN** the test is reviewed for contract coverage
- **THEN** the test MUST assert on the returned value or error content, not merely check `err != nil`

### Requirement: Contract Coverage Gate Compliance

The project MUST maintain an average contract coverage of at least 50% across all analyzed functions when measured by `gaze report` with `--min-contract-coverage=50`.

#### Scenario: Contract coverage passes CI gate
- **GIVEN** all source files are analyzed with `gaze report ./... --min-contract-coverage=50`
- **WHEN** the command executes
- **THEN** it MUST exit with code 0 (average contract coverage ≥ 50%)

#### Scenario: Existing tests enhanced with contract assertions
- **GIVEN** a test that calls a function but does not assert on the function's contractual return values
- **WHEN** contract assertions are added
- **THEN** the test's contract coverage for that function MUST increase to at least 75%

### Requirement: Test Isolation

All new and modified tests MUST be executable in isolation without requiring external services. Tests MUST NOT make real network calls to Slack, Google, or Chrome.

#### Scenario: Tests run without network access
- **GIVEN** a test file in any package
- **WHEN** `go test -count=1` is run for that package with no network access
- **THEN** all tests MUST pass

#### Scenario: External dependencies are mocked
- **GIVEN** a function under test that calls an external API (Slack, Google Drive, Chrome CDP)
- **WHEN** a test is written for that function
- **THEN** the test MUST use interface-based stubs or mocks, not real API calls

### Requirement: Behavioral Preservation

All refactoring MUST preserve existing public API signatures and behavior. No exported function signatures, types, or interfaces SHALL change.

#### Scenario: Existing tests pass after refactoring
- **GIVEN** the full test suite passes before a refactoring change
- **WHEN** a function is decomposed or restructured
- **THEN** all existing tests MUST continue to pass without modification

#### Scenario: Public API unchanged
- **GIVEN** an exported function, type, or interface in any package
- **WHEN** the implementation is refactored
- **THEN** the function signature, type definition, or interface MUST remain identical

## MODIFIED Requirements

_None — this change adds quality infrastructure without modifying existing behavioral requirements._

## REMOVED Requirements

_None._
