## Why

The current Slack API rate limiting strategy uses two patterns, both suboptimal:

1. **Burst-then-wait** — `GetAllMessages` and `GetAllReplies` fire requests as fast as possible with zero delay between pages. When a 429 hits, the `Retry-After` penalty is typically 10-30 seconds of dead time plus a wasted round-trip. For a conversation with 50 pages of history, hitting even 3 rate limits adds 30-90 seconds of pure waste.

2. **Hardcoded fixed delays** — The resolver functions (`LoadUsersForConversations`, `LoadChannels`, `fetchConversationMembers`) use `time.After(200ms)` and `time.After(500ms)` between calls regardless of the endpoint's actual rate limit. Slack's rate limits vary by tier: `users.info` allows ~100 req/min (600ms interval), but `users.list` allows only ~20 req/min (3s interval). A 200ms delay on `users.info` is too aggressive (triggers 429s), while a 500ms delay on `conversations.members` (50 req/min) is slower than necessary.

Neither pattern adapts to the actual Slack API tier limits. The result is that exports are simultaneously slower than necessary (fixed delays on high-limit endpoints) and prone to penalties (no delays on low-limit endpoints).

## What Changes

Replace both patterns with a single per-endpoint rate limiter that paces requests at the correct interval for each Slack API tier. The limiter is injected into the `slackapi.Client` and called automatically before every request, so callers don't need to manage their own delays.

Phase 1 (this change): Static per-endpoint intervals based on Slack's documented tier limits, with adaptive backoff when 429s occur.

Phase 2 (future): Dynamic rate discovery by observing response headers and 429 frequency to auto-tune intervals for undocumented or changed limits.

## Capabilities

### New Capabilities
- `RateLimiter`: A per-endpoint rate limiter in `pkg/slackapi/` that paces requests to stay within Slack's tier limits. Tracks the last request time per endpoint and injects the minimum delay needed before the next call.
- Adaptive backoff: When a 429 is received despite pacing, the limiter doubles the interval for that endpoint and uses the `Retry-After` header as a floor.
- Gradual recovery: After successful requests following a backoff, the interval decays back toward the baseline.

### Modified Capabilities
- `slackapi.Client`: The `request()` method calls `limiter.Wait(endpoint)` before every API call. The existing 429 retry loop in `request()` is simplified since 429s should become rare.
- `GetAllMessages` / `GetAllReplies`: Remove the inline 429 handling loops — the `request()` method handles retries centrally.
- `parser.UserResolver.LoadUsersForConversations`: Remove hardcoded `time.After(200ms)` and `time.After(500ms)` delays — pacing is handled by the rate limiter in the client.
- `parser.UserResolver.LoadUsers`: Remove hardcoded pagination throttle — same reason.
- `parser.ChannelResolver.LoadChannels`: Remove hardcoded pagination throttle.
- `parser.fetchConversationMembers`: Remove hardcoded inter-page throttle.

### Removed Capabilities
- None

## Impact

- **Performance**: Exports should be faster for large conversations because requests are paced at the maximum safe rate instead of being delayed by fixed intervals or penalized by 429 retries. The improvement is most significant for endpoints with high tier limits (users.info at Tier 4) where the current 200ms delay is replaced by the correct 600ms interval, eliminating 429 penalties entirely.
- **Reliability**: Fewer 429 errors means fewer retry cycles and lower risk of exhausting the 3-retry limit.
- **Stealth**: Consistent pacing looks more like natural user behavior than burst patterns, which aligns with the constitution's stealth principles.
- **Files changed**: `pkg/slackapi/client.go` (rate limiter integration), `pkg/slackapi/ratelimit.go` (new file), `pkg/parser/resolver.go` (remove hardcoded delays)
- **No behavioral changes**: Message content, export format, folder structure, and all public APIs remain identical. Only the timing between Slack API calls changes.
- **No new dependencies**: Uses only the standard library (`sync`, `time`).

## Constitution Alignment

Assessed against the get-out project constitution.

### I. Session-Driven Extraction

**Assessment**: N/A

This change does not alter the session-driven extraction strategy. Browser session handling, token extraction, and API mimicry behavior remain unchanged. The rate limiter only affects the timing between API calls, not how authentication or sessions work.

### II. Go-First Architecture

**Assessment**: PASS

The rate limiter is pure Go using only standard library types (`sync.Mutex`, `time.Duration`, `time.Time`). No external dependencies are introduced. The single-binary deployment model is preserved.

### III. Stealth & Reliability

**Assessment**: PASS

This change directly improves stealth: consistent pacing at documented tier limits mimics natural API usage patterns better than burst-then-429 behavior. Reliability improves because 429 errors (which risk session invalidation if frequent) are avoided rather than reacted to. The constitution's mandate for exponential backoff is preserved — the limiter uses adaptive backoff when 429s do occur.

### IV. Testability

**Assessment**: PASS

The `RateLimiter` is a standalone type with no external dependencies. It can be tested in isolation with deterministic time control. The `SlackAPI` interface (already introduced in `pkg/parser`) means resolver tests don't need to change. The rate limiter's `Wait()` method is context-aware, so tests can use cancelled contexts for fast execution.
