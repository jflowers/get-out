## 1. Create RateLimiter Type

- [x] 1.1 Create `pkg/slackapi/ratelimit.go` with `RateLimiter` struct, `endpointState` struct, and `NewRateLimiter(defaults map[string]time.Duration)` constructor
- [x] 1.2 Implement `Wait(ctx context.Context, endpoint string) error` — blocks until the endpoint's interval has elapsed since its last request; returns `ctx.Err()` if context is cancelled
- [x] 1.3 Implement `RecordSuccess(endpoint string)` — after a successful request, decay the interval by 10% toward baseline (never below baseline); reset backoff counter when baseline is reached
- [x] 1.4 Implement `RecordRateLimit(endpoint string, retryAfter time.Duration)` — set interval to `max(current * 2, retryAfter)`, increment backoff counter
- [x] 1.5 Define `DefaultTierIntervals() map[string]time.Duration` with the endpoint-to-interval mapping: `auth.test`=600ms, `conversations.history`=1200ms, `conversations.replies`=1200ms, `conversations.info`=1200ms, `conversations.members`=1200ms, `conversations.list`=3000ms, `users.info`=600ms, `users.list`=3000ms; unknown endpoints default to 1200ms

## 2. Test RateLimiter

- [x] 2.1 Test `Wait` — first call returns immediately; second call blocks for at least the baseline interval; verify with time measurement
- [x] 2.2 Test independent endpoint pacing — `Wait` on endpoint A does not delay endpoint B
- [x] 2.3 Test context cancellation — `Wait` returns `context.Canceled` when context is cancelled during a long wait
- [x] 2.4 Test `RecordRateLimit` — interval doubles after 429; `retryAfter` is used as floor; consecutive 429s compound
- [x] 2.5 Test `RecordSuccess` recovery — interval decays by 10% per success; never goes below baseline; ~7 successes return to baseline from 2x elevation
- [x] 2.6 Test unknown endpoint — defaults to 1200ms interval
- [x] 2.7 Test concurrent access — multiple goroutines calling `Wait` on the same endpoint serialize correctly (no data races with `-race`)

## 3. Integrate RateLimiter into Client

- [x] 3.1 Add `limiter *RateLimiter` field to `Client` struct
- [x] 3.2 Initialize `limiter` with `NewRateLimiter(DefaultTierIntervals())` in `NewBrowserClient` and `NewAPIClient`
- [x] 3.3 Add `WithRateLimiter(rl *RateLimiter) ClientOption` for test injection
- [x] 3.4 Modify `request()` to call `c.limiter.Wait(ctx, endpoint)` before `doRequest()`; call `c.limiter.RecordSuccess(endpoint)` on success; call `c.limiter.RecordRateLimit(endpoint, rle.RetryAfter)` on 429 — remove the inline `time.After(rle.RetryAfter)` sleep
- [x] 3.5 Add debug logging in `Wait()` when a delay is applied: log the endpoint, wait duration, and whether the interval is elevated

## 4. Simplify GetAllMessages and GetAllReplies

- [x] 4.1 Remove the inline 429 retry loop from `GetAllMessages` — let `request()` (via `GetConversationHistory`) handle all retries and rate limiting
- [x] 4.2 Remove the inline 429 retry loop from `GetAllReplies` — let `request()` (via `GetConversationReplies`) handle all retries and rate limiting
- [x] 4.3 Verify existing tests still pass after the simplification

## 5. Remove Hardcoded Delays from Resolver

- [x] 5.1 Remove `time.After(500 * time.Millisecond)` from `fetchConversationMembers` (inter-page throttle)
- [x] 5.2 Remove `time.After(500 * time.Millisecond)` from `LoadUsersForConversations` (inter-conversation throttle)
- [x] 5.3 Remove `time.After(200 * time.Millisecond)` from `LoadUsersForConversations` (user info fetch throttle)
- [x] 5.4 Remove `time.After(500 * time.Millisecond)` from `LoadChannels` (inter-page throttle) if present
- [x] 5.5 Remove `time.After(1200 * time.Millisecond)` from `LoadUsers` (inter-page throttle) if present
- [x] 5.6 Update resolver tests that relied on `time.After` delays for context cancellation — these tests use mock `SlackAPI` implementations and should not be affected, but verify

## 6. Verification

- [x] 6.1 Run full test suite: `go test -race -count=1 ./...` — all tests MUST pass
- [x] 6.2 Run gaze CRAPload check: verify CRAPload ≤ 10 is maintained (new code should have tests, not increase CRAPload)
- [x] 6.3 Verify no new external dependencies introduced (constitution: Go-First Architecture, Composability First)
- [x] 6.4 Verify `--debug` output shows rate limiter pacing messages during a real export (constitution: Observable Quality)
- [x] 6.5 Verify constitution alignment: all four principles (Autonomous Collaboration, Composability First, Observable Quality, Testability) pass per the proposal assessment
