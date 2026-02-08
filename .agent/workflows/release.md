---
description: How to create a new release of get-out
---

# Release Process

## Prerequisites
- All CI checks passing on the branch
- Changes merged to `main`

## Steps

### 1. Merge feature branch to main
// turbo
```bash
git checkout main && git pull origin main
```

```bash
git merge --no-ff 001-slack-message-export -m "Merge 001-slack-message-export: Slack message export to Google Docs"
```

// turbo
```bash
git push origin main
```

### 2. Tag the release

Use semantic versioning (`vMAJOR.MINOR.PATCH`):
- **Major** (v2.0.0): Breaking changes
- **Minor** (v1.1.0): New features, backward compatible
- **Patch** (v1.0.1): Bug fixes only

```bash
git tag -a v1.0.0 -m "v1.0.0: Slack message export to Google Docs"
```

// turbo
```bash
git push origin v1.0.0
```

### 3. Verify release

GoReleaser automatically:
- Builds binaries for macOS + Linux (amd64/arm64)
- Creates a GitHub Release with auto-generated notes
- Groups commits by type (Features, Bug Fixes, Documentation)

Check: https://github.com/jflowers/get-out/releases

### 4. Post-release

// turbo
```bash
git checkout 001-slack-message-export
```

Continue development on feature branches.
