package slackapi

import (
	"context"
	"sync"
	"testing"
	"time"
)

func testLimiter() *RateLimiter {
	return NewRateLimiter(map[string]time.Duration{
		"fast": 50 * time.Millisecond,
		"slow": 200 * time.Millisecond,
	})
}

// 2.1 — Wait: first call immediate, second call blocks
func TestRateLimiter_Wait_FirstCallImmediate(t *testing.T) {
	rl := testLimiter()

	start := time.Now()
	err := rl.Wait(context.Background(), "fast")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed > 10*time.Millisecond {
		t.Errorf("first call took %v, expected immediate", elapsed)
	}
}

func TestRateLimiter_Wait_SecondCallBlocks(t *testing.T) {
	rl := testLimiter()

	_ = rl.Wait(context.Background(), "fast")

	start := time.Now()
	err := rl.Wait(context.Background(), "fast")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed < 40*time.Millisecond {
		t.Errorf("second call took %v, expected at least ~50ms", elapsed)
	}
}

// 2.2 — Independent endpoint pacing
func TestRateLimiter_IndependentEndpoints(t *testing.T) {
	rl := testLimiter()

	// Call "slow" endpoint first
	_ = rl.Wait(context.Background(), "slow")

	// Calling "fast" endpoint should not be delayed by "slow"
	start := time.Now()
	err := rl.Wait(context.Background(), "fast")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed > 10*time.Millisecond {
		t.Errorf("fast endpoint delayed by %v after slow endpoint call, expected immediate", elapsed)
	}
}

// 2.3 — Context cancellation during wait
func TestRateLimiter_Wait_ContextCancelled(t *testing.T) {
	rl := NewRateLimiter(map[string]time.Duration{
		"blocked": 5 * time.Second,
	})

	// First call to establish lastRequest
	_ = rl.Wait(context.Background(), "blocked")

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	err := rl.Wait(ctx, "blocked")
	elapsed := time.Since(start)

	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	if elapsed > 200*time.Millisecond {
		t.Errorf("wait took %v, expected quick cancellation", elapsed)
	}
}

// 2.4 — RecordRateLimit behavior
func TestRateLimiter_RecordRateLimit_DoublesInterval(t *testing.T) {
	rl := testLimiter()

	// Establish state
	_ = rl.Wait(context.Background(), "fast")

	// Record a 429 — should double from 50ms to 100ms
	rl.RecordRateLimit("fast", 0)

	rl.mu.Lock()
	s := rl.endpoints["fast"]
	got := s.interval
	rl.mu.Unlock()

	if got != 100*time.Millisecond {
		t.Errorf("interval after RecordRateLimit = %v, want 100ms", got)
	}
}

func TestRateLimiter_RecordRateLimit_RetryAfterFloor(t *testing.T) {
	rl := testLimiter()
	_ = rl.Wait(context.Background(), "fast")

	// retryAfter (500ms) > doubled (100ms), so retryAfter wins
	rl.RecordRateLimit("fast", 500*time.Millisecond)

	rl.mu.Lock()
	got := rl.endpoints["fast"].interval
	rl.mu.Unlock()

	if got != 500*time.Millisecond {
		t.Errorf("interval = %v, want 500ms (retryAfter floor)", got)
	}
}

func TestRateLimiter_RecordRateLimit_Compounds(t *testing.T) {
	rl := testLimiter()
	_ = rl.Wait(context.Background(), "fast")

	// First 429: 50ms → 100ms
	rl.RecordRateLimit("fast", 0)
	// Second 429: 100ms → 200ms
	rl.RecordRateLimit("fast", 0)

	rl.mu.Lock()
	got := rl.endpoints["fast"].interval
	backoffs := rl.endpoints["fast"].backoffs
	rl.mu.Unlock()

	if got != 200*time.Millisecond {
		t.Errorf("interval after 2 backoffs = %v, want 200ms", got)
	}
	if backoffs != 2 {
		t.Errorf("backoffs = %d, want 2", backoffs)
	}
}

// 2.5 — RecordSuccess recovery
func TestRateLimiter_RecordSuccess_DecaysToBaseline(t *testing.T) {
	rl := testLimiter()
	_ = rl.Wait(context.Background(), "fast")

	// Elevate to 2x baseline
	rl.RecordRateLimit("fast", 0) // 50ms → 100ms

	// ~7 successes should bring it back to baseline
	for i := 0; i < 10; i++ {
		rl.RecordSuccess("fast")
	}

	rl.mu.Lock()
	got := rl.endpoints["fast"].interval
	backoffs := rl.endpoints["fast"].backoffs
	rl.mu.Unlock()

	if got != 50*time.Millisecond {
		t.Errorf("interval after recovery = %v, want baseline 50ms", got)
	}
	if backoffs != 0 {
		t.Errorf("backoffs after full recovery = %d, want 0", backoffs)
	}
}

func TestRateLimiter_RecordSuccess_NeverBelowBaseline(t *testing.T) {
	rl := testLimiter()
	_ = rl.Wait(context.Background(), "fast")

	// Already at baseline — success should not decrease further
	for i := 0; i < 5; i++ {
		rl.RecordSuccess("fast")
	}

	rl.mu.Lock()
	got := rl.endpoints["fast"].interval
	rl.mu.Unlock()

	if got != 50*time.Millisecond {
		t.Errorf("interval = %v, want baseline 50ms (should not go below)", got)
	}
}

// 2.6 — Unknown endpoint defaults to fallback
func TestRateLimiter_UnknownEndpoint(t *testing.T) {
	rl := testLimiter()

	_ = rl.Wait(context.Background(), "unknown.method")

	rl.mu.Lock()
	s := rl.endpoints["unknown.method"]
	rl.mu.Unlock()

	if s == nil {
		t.Fatal("unknown endpoint not tracked")
	}
	if s.baseline != 1200*time.Millisecond {
		t.Errorf("unknown endpoint baseline = %v, want 1200ms", s.baseline)
	}
}

// 2.7 — Concurrent access safety
func TestRateLimiter_ConcurrentAccess(t *testing.T) {
	rl := NewRateLimiter(map[string]time.Duration{
		"shared": 10 * time.Millisecond,
	})

	var wg sync.WaitGroup
	const goroutines = 10

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				if err := rl.Wait(context.Background(), "shared"); err != nil {
					t.Errorf("Wait error: %v", err)
					return
				}
				rl.RecordSuccess("shared")
			}
		}()
	}

	wg.Wait()

	rl.mu.Lock()
	s := rl.endpoints["shared"]
	rl.mu.Unlock()

	if s == nil {
		t.Fatal("shared endpoint not tracked")
	}
	// Should still be at baseline after all successes
	if s.interval != 10*time.Millisecond {
		t.Errorf("interval = %v, want baseline 10ms", s.interval)
	}
}

// Additional: DefaultTierIntervals returns expected mapping
func TestDefaultTierIntervals(t *testing.T) {
	tiers := DefaultTierIntervals()

	expected := map[string]time.Duration{
		"auth.test":             600 * time.Millisecond,
		"users.info":            600 * time.Millisecond,
		"conversations.history": 1200 * time.Millisecond,
		"conversations.replies": 1200 * time.Millisecond,
		"conversations.info":    1200 * time.Millisecond,
		"conversations.members": 1200 * time.Millisecond,
		"conversations.list":    3000 * time.Millisecond,
		"users.list":            3000 * time.Millisecond,
	}

	for endpoint, want := range expected {
		got, ok := tiers[endpoint]
		if !ok {
			t.Errorf("missing endpoint %s", endpoint)
			continue
		}
		if got != want {
			t.Errorf("tiers[%s] = %v, want %v", endpoint, got, want)
		}
	}

	if len(tiers) != len(expected) {
		t.Errorf("tier count = %d, want %d", len(tiers), len(expected))
	}
}

// ---------------------------------------------------------------------------
// Phase 2: Debug path branch test
// ---------------------------------------------------------------------------

func TestRateLimiter_Wait_DebugPath(t *testing.T) {
	rl := testLimiter()
	rl.SetDebug(true)

	// First call — immediate return
	err := rl.Wait(context.Background(), "fast")
	if err != nil {
		t.Fatalf("first Wait() = %v, want nil", err)
	}

	// Second call — triggers debug logging and wait
	err = rl.Wait(context.Background(), "fast")
	// Contract assertion: debug path returns nil (no error)
	if err != nil {
		t.Errorf("second Wait() with debug = %v, want nil", err)
	}
}

// ---------------------------------------------------------------------------
// Phase 3: Confidence-79 gap-specific test — observable effect
// ---------------------------------------------------------------------------

func TestRecordRateLimit_ObservableEffect(t *testing.T) {
	rl := testLimiter() // "fast" = 50ms

	// Establish baseline — first Wait is immediate
	err := rl.Wait(context.Background(), "fast")
	if err != nil {
		t.Fatalf("first Wait: %v", err)
	}

	// Record a rate limit — this doubles the interval from 50ms to 100ms
	rl.RecordRateLimit("fast", 0)

	// The next Wait should block for ~100ms (the doubled interval)
	start := time.Now()
	err = rl.Wait(context.Background(), "fast")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Wait after RecordRateLimit: %v", err)
	}

	// Contract assertion: the doubled interval is observable through Wait
	if elapsed < 80*time.Millisecond {
		t.Errorf("Wait after RecordRateLimit took %v, expected >= 80ms (doubled 50ms interval)", elapsed)
	}
}
