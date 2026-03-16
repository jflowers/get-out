## Context

The Slack API enforces rate limits organized by tier. Each API method belongs to a tier that defines how many requests per minute are allowed. The current codebase handles rate limits in two inconsistent ways:

1. **No pacing + reactive retry** in `slackapi.Client.request()` — fires requests immediately, catches 429 errors, waits `Retry-After` seconds, retries up to 3 times. This is duplicated in `GetAllMessages` and `GetAllReplies` with their own inline 429 loops.

2. **Hardcoded fixed delays** in `parser/resolver.go` — `time.After(200ms)` between `users.info` calls, `time.After(500ms)` between pagination pages and between conversations. These values don't correspond to any actual Slack tier limit.

The result: burst patterns trigger unnecessary 429 penalties on high-volume endpoints, while fixed delays waste time on endpoints that allow higher throughput.

## Goals / Non-Goals

### Goals
- Pace all Slack API requests at the correct per-endpoint interval to avoid 429 errors
- Centralize rate limiting in a single component so callers don't manage their own delays
- Adapt when 429s occur (increase interval) and recover when they stop (decrease toward baseline)
- Maintain testability — the rate limiter must be testable without real Slack API calls (constitution principle IV)
- Preserve all existing behavior except timing between requests

### Non-Goals
- Dynamic rate discovery from response headers (Phase 2 — not this change)
- Per-workspace or per-token rate tracking (Slack rate limits are per-token, but we only use one token at a time)
- Rate limiting for Google Drive/Docs API calls (separate concern, handled by `gdrive.retryOnRateLimit`)
- Changing the `SlackAPI` interface in `pkg/parser` — the rate limiter is internal to the client

## Decisions

### 1. New `RateLimiter` type in `pkg/slackapi/ratelimit.go`

The rate limiter is a standalone type with no external dependencies, satisfying the Composability First and Testability principles.

```go
type RateLimiter struct {
    mu        sync.Mutex
    endpoints map[string]*endpointState
    defaults  map[string]time.Duration // endpoint → baseline interval
}

type endpointState struct {
    lastRequest time.Time
    interval    time.Duration // current interval (may be elevated after 429)
    baseline    time.Duration // configured minimum interval for this tier
    backoffs    int           // consecutive 429 count
}
```

**Key methods:**
- `Wait(ctx, endpoint) error` — blocks until the endpoint's interval has elapsed since the last request. Returns `ctx.Err()` if the context is cancelled while waiting.
- `RecordSuccess(endpoint)` — after a successful request, decays the interval back toward the baseline if it was elevated by a prior 429.
- `RecordRateLimit(endpoint, retryAfter)` — after a 429, sets the interval to `max(current * 2, retryAfter)` and increments the backoff counter.

### 2. Endpoint-to-tier mapping

Based on Slack's documented API rate limits:

| Endpoint | Slack Tier | Requests/min | Baseline Interval |
|----------|-----------|-------------|-------------------|
| `auth.test` | Tier 4 | ~100 | 600ms |
| `conversations.history` | Tier 3 | ~50 | 1200ms |
| `conversations.replies` | Tier 3 | ~50 | 1200ms |
| `conversations.info` | Tier 3 | ~50 | 1200ms |
| `conversations.members` | Tier 3 | ~50 | 1200ms |
| `conversations.list` | Tier 2 | ~20 | 3000ms |
| `users.info` | Tier 4 | ~100 | 600ms |
| `users.list` | Tier 2 | ~20 | 3000ms |

Unknown endpoints default to Tier 3 (1200ms) as a safe middle ground.

These intervals include a 10% safety margin over the theoretical minimum to account for clock skew and concurrent requests from other clients using the same token.

### 3. Integration into `Client.request()`

The rate limiter is called at the start of `request()`, before `doRequest()`:

```go
func (c *Client) request(ctx context.Context, method, endpoint string, params url.Values, result interface{}) error {
    const maxRetries = 3

    for attempt := 0; attempt <= maxRetries; attempt++ {
        // Wait for rate limit clearance
        if err := c.limiter.Wait(ctx, endpoint); err != nil {
            return err
        }

        err := c.doRequest(ctx, method, endpoint, params, result)
        if err == nil {
            c.limiter.RecordSuccess(endpoint)
            return nil
        }

        rle, ok := err.(*RateLimitError)
        if !ok || attempt == maxRetries {
            return err
        }

        c.limiter.RecordRateLimit(endpoint, rle.RetryAfter)
        // Wait is handled by limiter.Wait on the next iteration
    }

    return fmt.Errorf("exhausted retries for %s", endpoint)
}
```

This eliminates the `time.After(rle.RetryAfter)` sleep in the retry loop — the limiter's `Wait()` on the next iteration handles the delay, using the elevated interval set by `RecordRateLimit`.

### 4. Remove inline 429 handling from GetAllMessages / GetAllReplies

Currently `GetAllMessages` and `GetAllReplies` have their own 429 retry loops that duplicate the logic in `request()`. These are removed — all 429 handling flows through `request()` and the rate limiter.

### 5. Remove hardcoded delays from parser/resolver.go

All `time.After(200ms)`, `time.After(500ms)`, and `time.After(1200ms)` calls in `resolver.go` are removed. The rate limiter in the `slackapi.Client` handles pacing transparently. The resolver functions simply call the `SlackAPI` interface methods, which internally pace via the limiter.

### 6. Adaptive backoff and recovery

When a 429 occurs:
- `RecordRateLimit(endpoint, retryAfter)` sets `interval = max(interval * 2, retryAfter)` and increments `backoffs`.
- The next `Wait()` call for that endpoint will block for the elevated interval.

When requests succeed after a backoff:
- `RecordSuccess(endpoint)` decreases `interval` by 10% per successful call, but never below the `baseline`.
- After ~7 consecutive successes, the interval returns to baseline (`0.9^7 ≈ 0.48`, so the elevated interval halves).

### 7. Client construction

The rate limiter is created automatically in `NewBrowserClient` and `NewAPIClient`. A `WithRateLimiter` option allows tests to inject a custom limiter (or a no-op limiter for tests that don't care about pacing).

```go
func NewBrowserClient(token, cookie string, opts ...ClientOption) *Client {
    c := &Client{
        // ...
        limiter: NewRateLimiter(DefaultTierIntervals()),
    }
    for _, opt := range opts {
        opt(c)
    }
    return c
}
```

### 8. Observable Quality

The rate limiter logs a debug message whenever it waits, including the endpoint, wait duration, and whether the interval is elevated due to a prior 429. This satisfies the Observable Quality principle — operators can see the pacing behavior in `--debug` output.

## Risks / Trade-offs

### Slower initial requests for high-volume endpoints

The first request to each endpoint is unthrottled (no prior request to pace against). But subsequent requests wait at least the baseline interval. For endpoints like `users.info` (600ms), this is faster than the current 200ms delay that triggers 429s followed by 10s+ penalties.

**Trade-off accepted:** Slightly slower sustained throughput vs. zero 429 penalties.

### Tier limits may change

Slack could change their tier assignments or rate limits without notice. The hardcoded baseline intervals would become stale.

**Mitigation:** The adaptive backoff handles this — if a 429 occurs, the limiter automatically increases the interval. The baseline is a starting point, not a ceiling. Phase 2 (dynamic discovery) would address this more completely.

### Parallel export interaction

With `--parallel N`, multiple goroutines share the same `Client` and therefore the same `RateLimiter`. The limiter is mutex-protected so concurrent access is safe. However, N goroutines all pacing on `conversations.history` (1200ms interval) means effective throughput is 1200ms * 1 = one request per 1.2s regardless of concurrency, since they serialize through `Wait()`.

**Trade-off accepted:** This is correct behavior — Slack's rate limit is per-token, not per-goroutine. Parallelism helps with the Google Drive API calls and local processing, not Slack API throughput.

### Removing resolver delays changes test timing

Tests for `LoadUsersForConversations` that relied on `time.After(200-500ms)` delays for context cancellation scenarios will need adjustment. The rate limiter is injected via the client, so test clients can use a no-op limiter for fast execution.

**Mitigation:** Tests already use mock `SlackAPI` implementations that don't go through the rate limiter. Only tests that use real `Client` instances need updating.
