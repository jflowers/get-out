# Feature Specification: CI/CD with GitHub Actions

**Feature Branch**: `002-ci-cd`  
**Created**: 2026-02-07  
**Status**: Draft  
**Input**: User request: "Setup CI/CD with GitHub. Release should auto-generate release notes from commit messages."

## User Scenarios & Testing

### User Story 1 - Continuous Integration (Priority: P1)

As a developer, I want every push and pull request to automatically build, test, and lint
so that I catch regressions before merging.

**Acceptance Scenarios**:

1. **Given** a push to any branch, **When** GitHub Actions runs, **Then** the project builds, passes `go vet`, and all tests pass
2. **Given** a pull request, **When** CI runs, **Then** the PR shows a check status (pass/fail) before merge

---

### User Story 2 - Automated Releases with Notes (Priority: P1)

As a maintainer, I want pushing a semver tag (e.g., `v1.0.0`) to automatically create a
GitHub Release with cross-platform binaries and auto-generated release notes from commit
messages since the last tag.

**Acceptance Scenarios**:

1. **Given** I push a tag like `v1.0.0`, **When** GitHub Actions runs, **Then** a GitHub Release is created with macOS (amd64/arm64) and Linux (amd64/arm64) binaries
2. **Given** the release is created, **When** I view it on GitHub, **Then** the release notes contain commit messages grouped by type (feat, fix, docs, etc.)

---

## Success Criteria

- [ ] SC-001: Push to any branch triggers build + test + vet
- [ ] SC-002: PR shows CI check status
- [ ] SC-003: Tag push creates GitHub Release with binaries
- [ ] SC-004: Release notes auto-generated from commit messages
- [ ] SC-005: Binaries built for macOS (amd64/arm64) + Linux (amd64/arm64)

## Non-Goals

- Docker image publishing (not needed for a CLI tool)
- Deployment to any hosting service
- Code coverage reporting (can be added later)
