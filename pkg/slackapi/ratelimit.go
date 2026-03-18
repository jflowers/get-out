package slackapi

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"
)

// endpointState tracks the pacing state for a single API endpoint.
type endpointState struct {
	lastRequest time.Time
	interval    time.Duration // current interval (may be elevated after 429)
	baseline    time.Duration // configured minimum interval for this tier
	backoffs    int           // consecutive 429 count
}

// RateLimiter paces Slack API requests per endpoint to avoid 429 responses.
// Each endpoint has a baseline interval derived from its Slack API tier.
// When a 429 is received, the interval is doubled. On success after a backoff,
// the interval decays back toward the baseline.
type RateLimiter struct {
	mu        sync.Mutex
	endpoints map[string]*endpointState
	defaults  map[string]time.Duration // endpoint → baseline interval
	fallback  time.Duration            // interval for unknown endpoints
	debug     bool                     // whether to log debug messages
}

// NewRateLimiter creates a rate limiter with the given per-endpoint baseline intervals.
// Unknown endpoints use the fallback interval (1200ms).
func NewRateLimiter(defaults map[string]time.Duration) *RateLimiter {
	return &RateLimiter{
		endpoints: make(map[string]*endpointState),
		defaults:  defaults,
		fallback:  1200 * time.Millisecond,
	}
}

// SetDebug enables or disables debug logging for the rate limiter.
func (rl *RateLimiter) SetDebug(debug bool) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.debug = debug
}

// getOrCreate returns the endpoint state, creating it with the baseline if needed.
func (rl *RateLimiter) getOrCreate(endpoint string) *endpointState {
	if s, ok := rl.endpoints[endpoint]; ok {
		return s
	}
	baseline := rl.fallback
	if d, ok := rl.defaults[endpoint]; ok {
		baseline = d
	}
	s := &endpointState{
		interval: baseline,
		baseline: baseline,
	}
	rl.endpoints[endpoint] = s
	return s
}

// Wait blocks until the endpoint's rate-limit interval has elapsed since its
// last request. It mutates the endpoint's lastRequest timestamp to reserve a
// time slot, even before the wait completes, preventing concurrent callers
// from overlapping.
//
// If no wait is needed (enough time has already elapsed), it returns nil
// immediately without blocking. If debug mode is enabled, wait durations are
// logged to stderr.
//
// Returns nil on success. Returns ctx.Err() if the context is cancelled while
// waiting. The endpoint state is updated regardless of whether the wait
// completes or is cancelled.
func (rl *RateLimiter) Wait(ctx context.Context, endpoint string) error {
	rl.mu.Lock()
	s := rl.getOrCreate(endpoint)
	elapsed := time.Since(s.lastRequest)
	wait := s.interval - elapsed
	if wait < 0 {
		wait = 0
	}
	s.lastRequest = time.Now().Add(wait)
	debug := rl.debug
	interval := s.interval
	elevated := s.interval > s.baseline
	rl.mu.Unlock()

	if wait <= 0 {
		return nil
	}

	if debug {
		msg := fmt.Sprintf("  [rate-limit] %s: waiting %v", endpoint, wait.Round(time.Millisecond))
		if elevated {
			msg += fmt.Sprintf(" (elevated from %v baseline)", interval.Round(time.Millisecond))
		}
		fmt.Fprintln(os.Stderr, msg)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(wait):
		return nil
	}
}

// RecordSuccess decays the interval by 10% toward the baseline after a successful request.
// When the interval reaches the baseline, the backoff counter is reset.
func (rl *RateLimiter) RecordSuccess(endpoint string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	s := rl.getOrCreate(endpoint)
	if s.interval <= s.baseline {
		s.backoffs = 0
		return
	}

	// Decay by 10%
	s.interval = time.Duration(float64(s.interval) * 0.9)
	if s.interval <= s.baseline {
		s.interval = s.baseline
		s.backoffs = 0
	}
}

// RecordRateLimit elevates the interval for the endpoint after a 429 response.
// The new interval is max(current * 2, retryAfter).
func (rl *RateLimiter) RecordRateLimit(endpoint string, retryAfter time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	s := rl.getOrCreate(endpoint)
	s.backoffs++

	doubled := s.interval * 2
	if retryAfter > doubled {
		s.interval = retryAfter
	} else {
		s.interval = doubled
	}
}

// NoOpRateLimiter returns a rate limiter with zero-delay intervals for all
// endpoints. Use this in tests to avoid real pacing delays.
func NoOpRateLimiter() *RateLimiter {
	return &RateLimiter{
		endpoints: make(map[string]*endpointState),
		defaults:  make(map[string]time.Duration),
		fallback:  0,
	}
}

// DefaultTierIntervals returns the per-endpoint baseline intervals based on
// Slack's documented API rate limit tiers. Intervals include a 10% safety margin.
//
// Tier 2 (~20 req/min): 3000ms
// Tier 3 (~50 req/min): 1200ms
// Tier 4 (~100 req/min): 600ms
func DefaultTierIntervals() map[string]time.Duration {
	return map[string]time.Duration{
		// Tier 4
		"auth.test":  600 * time.Millisecond,
		"users.info": 600 * time.Millisecond,

		// Tier 3
		"conversations.history": 1200 * time.Millisecond,
		"conversations.replies": 1200 * time.Millisecond,
		"conversations.info":    1200 * time.Millisecond,
		"conversations.members": 1200 * time.Millisecond,

		// Tier 2
		"conversations.list": 3000 * time.Millisecond,
		"users.list":         3000 * time.Millisecond,
	}
}
