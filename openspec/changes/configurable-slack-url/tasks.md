## 1. Add Config Field and Validation

- [x] 1.1 Add `SlackWorkspaceURL string \`json:"slackWorkspaceUrl,omitempty"\`` field to `Settings` struct in `pkg/config/types.go`
- [x] 1.2 Set `SlackWorkspaceURL: "https://app.slack.com"` in `DefaultSettings()` in `pkg/config/types.go`
- [x] 1.3 Add `ValidateSlackURL(rawURL string) error` function in `pkg/config/config.go` — parse URL, require `https` scheme, require host is `slack.com` or `*.slack.com`
- [x] 1.4 Call `ValidateSlackURL` in `LoadSettings()` when `SlackWorkspaceURL` is non-empty; if empty, set to default

## 2. Tests for Config and Validation

- [x] 2.1 Add table-driven tests for `ValidateSlackURL` in `pkg/config/config_test.go` — cover: valid URLs (`https://app.slack.com`, `https://mycompany.slack.com`, `https://slack.com`), invalid scheme (`http://`), non-Slack host (`https://example.com`), spoofed host (`https://slack.com.evil.com`), malformed URL, empty string
- [x] 2.2 Add test for `LoadSettings` with `slackWorkspaceUrl` field present — verify it's loaded into struct
- [x] 2.3 Add test for `LoadSettings` with missing/empty `slackWorkspaceUrl` — verify default is applied
- [x] 2.4 Add test for `LoadSettings` with invalid `slackWorkspaceUrl` — verify error is returned

## 3. Use Configured URL in setup-browser

- [x] 3.1 Change `launchChrome` signature in `internal/cli/selfservice.go` from `(profilePath string, port int)` to `(profilePath string, port int, slackURL string)` and use `slackURL` instead of hardcoded `"https://app.slack.com"`
- [x] 3.2 Update `runSetupBrowser` to pass `settings.SlackWorkspaceURL` to `launchChrome`
- [x] 3.3 Update hint text in `evalSlackTargets()` (~line 626, ~881) to use configured URL instead of hardcoded `https://app.slack.com`
- [x] 3.4 Update sign-in instruction text (~line 827) to use configured URL
- [x] 3.5 Thread settings (or the URL string) through to any other location in `selfservice.go` that displays the hardcoded Slack URL

## 4. Update Documentation and Example Config

- [x] 4.1 Add `"slackWorkspaceUrl": "https://app.slack.com"` to `config/settings.json.example`
- [x] 4.2 Update `README.md` to document the new `slackWorkspaceUrl` setting
- [x] 4.3 Update `AGENTS.md` if settings schema section needs refreshing

## 5. Verify

- [x] 5.1 Run `go build ./...` to verify compilation
- [x] 5.2 Run `go test -race -count=1 ./...` to verify all tests pass
- [x] 5.3 Verify constitution alignment: Composability (setting is optional with default) and Testability (validation is pure function, tests added)
<!-- spec-review: passed -->
<!-- code-review: passed -->
