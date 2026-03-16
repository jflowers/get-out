## ADDED Requirements

### Requirement: Per-Endpoint Rate Pacing

The Slack API client MUST pace requests per endpoint at an interval that prevents 429 responses under normal conditions. Each endpoint MUST have a configured baseline interval derived from its Slack API tier.

#### Scenario: Pacing prevents 429 on sustained requests
- **GIVEN** a rate limiter configured with a 1200ms interval for `conversations.history`
- **WHEN** 10 consecutive `conversations.history` requests are made
- **THEN** each request after the first MUST be delayed by at least 1200ms from the previous one AND zero 429 errors SHOULD be returned

#### Scenario: First request is not delayed
- **GIVEN** a rate limiter with no prior requests for an endpoint
- **WHEN** a request is made to that endpoint
- **THEN** the request MUST proceed immediately with no delay

#### Scenario: Different endpoints are paced independently
- **GIVEN** a rate limiter with a 600ms interval for `users.info` and a 1200ms interval for `conversations.history`
- **WHEN** a `users.info` request is made followed immediately by a `conversations.history` request
- **THEN** the `conversations.history` request MUST NOT be delayed by the `users.info` pacing

#### Scenario: Unknown endpoints use a safe default
- **GIVEN** a rate limiter with no explicit configuration for an endpoint
- **WHEN** a request is made to that unknown endpoint
- **THEN** the limiter MUST pace it at the Tier 3 default interval (1200ms)

### Requirement: Adaptive Backoff on 429

When a 429 response is received despite pacing, the rate limiter MUST increase the interval for the affected endpoint and use the `Retry-After` header value as a minimum.

#### Scenario: 429 doubles the interval
- **GIVEN** a rate limiter with a current interval of 1200ms for an endpoint
- **WHEN** a 429 response is received with `Retry-After: 5`
- **THEN** the interval for that endpoint MUST be set to at least `max(2400ms, 5s)` = 5s

#### Scenario: Consecutive 429s compound the backoff
- **GIVEN** a rate limiter with a baseline of 1200ms that has already been backed off to 5s
- **WHEN** another 429 is received with `Retry-After: 3`
- **THEN** the interval MUST be set to at least `max(10s, 3s)` = 10s

### Requirement: Gradual Recovery After Backoff

After a backoff, the rate limiter MUST gradually reduce the interval back toward the baseline as successful requests are observed.

#### Scenario: Recovery after elevated interval
- **GIVEN** a rate limiter with baseline 1200ms and current interval elevated to 5s after a 429
- **WHEN** 7 consecutive successful requests are made
- **THEN** the interval MUST have decreased to within 10% of the baseline (approximately 1200ms)

#### Scenario: Recovery never goes below baseline
- **GIVEN** a rate limiter with baseline 1200ms and current interval at 1300ms (recovering)
- **WHEN** a successful request is made
- **THEN** the interval MUST NOT decrease below 1200ms

### Requirement: Context-Aware Waiting

The rate limiter's wait mechanism MUST respect context cancellation so that callers can abort long waits.

#### Scenario: Context cancelled during wait
- **GIVEN** a rate limiter with a 30s elevated interval for an endpoint
- **WHEN** a request is made and the context is cancelled after 1s
- **THEN** the wait MUST return immediately with `context.Canceled` error

### Requirement: Centralized Rate Limit Handling

All Slack API rate limit handling MUST flow through the rate limiter and the `request()` retry loop. Callers MUST NOT implement their own 429 retry or delay logic.

#### Scenario: GetAllMessages delegates rate limiting
- **GIVEN** a Slack client with a rate limiter
- **WHEN** `GetAllMessages` is called and the Slack API returns a 429 on the second page
- **THEN** the rate limiter's interval for `conversations.history` MUST be elevated AND the retry MUST be handled by `request()`, not by `GetAllMessages`

#### Scenario: Resolver functions have no delay logic
- **GIVEN** a resolver function calling `LoadUsersForConversations`
- **WHEN** the function fetches user info for 10 members
- **THEN** no `time.After` or `time.Sleep` calls MUST exist in the resolver code — all pacing is handled by the client's rate limiter

## MODIFIED Requirements

### Requirement: Slack API Retry Behavior

Previously: The `request()` method caught 429 errors, slept for `Retry-After` duration inline, and retried up to 3 times. `GetAllMessages` and `GetAllReplies` had their own duplicate 429 handling loops.

Now: The `request()` method calls `limiter.RecordRateLimit()` on 429, then loops back to `limiter.Wait()` which enforces the elevated interval. `GetAllMessages` and `GetAllReplies` delegate all error handling to `request()` and do not contain 429-specific logic.

### Requirement: Resolver Throttling

Previously: `LoadUsersForConversations` used `time.After(200ms)` between user info calls and `time.After(500ms)` between conversations. `LoadChannels` used `time.After(500ms)` between pages. `fetchConversationMembers` used `time.After(500ms)` between pages.

Now: All hardcoded delays are removed. Pacing is handled transparently by the rate limiter inside the `slackapi.Client`. Resolver functions call the `SlackAPI` interface methods without any timing concerns.

## REMOVED Requirements

_None._
