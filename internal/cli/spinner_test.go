package cli

import (
	"bytes"
	"strings"
	"sync"
	"testing"
	"time"
)

// 2.1 — NewStatusSpinner returns non-nil with expected defaults
func TestNewStatusSpinner(t *testing.T) {
	s := NewStatusSpinner()
	if s == nil {
		t.Fatal("NewStatusSpinner returned nil")
	}
	if len(s.frames) == 0 {
		t.Error("frames is empty")
	}
	if s.interval != 100*time.Millisecond {
		t.Errorf("interval = %v, want 100ms", s.interval)
	}
	if s.writer == nil {
		t.Error("writer is nil")
	}
	if s.active {
		t.Error("should not be active before Start()")
	}
}

// 2.2 — Start/Stop lifecycle
func TestStatusSpinner_StartStop(t *testing.T) {
	var buf bytes.Buffer
	s := NewStatusSpinner()
	s.writer = &buf
	s.interval = 10 * time.Millisecond // fast ticks for testing

	s.Start()
	if !s.active {
		t.Error("should be active after Start()")
	}

	// Let it tick a few times
	time.Sleep(50 * time.Millisecond)

	s.Stop()
	if s.active {
		t.Error("should not be active after Stop()")
	}

	// Verify it wrote something (spinner frames)
	output := buf.String()
	if len(output) == 0 {
		t.Error("spinner wrote nothing to buffer")
	}

	// Verify the line was cleared (ends with spaces + \r)
	if !strings.HasSuffix(output, "\r") {
		t.Error("spinner did not clear the line on Stop()")
	}
}

// 2.3 — Update changes the message
func TestStatusSpinner_Update(t *testing.T) {
	var buf bytes.Buffer
	s := NewStatusSpinner()
	s.writer = &buf
	s.interval = 10 * time.Millisecond

	s.Start()
	s.Update("loading users")
	time.Sleep(50 * time.Millisecond)
	s.Stop()

	output := buf.String()
	if !strings.Contains(output, "loading users") {
		t.Errorf("output does not contain updated message, got: %q", output)
	}
}

// 2.4 — Stop before Start does not panic
func TestStatusSpinner_StopBeforeStart(t *testing.T) {
	s := NewStatusSpinner()
	// Should not panic
	s.Stop()
	if s.active {
		t.Error("should not be active")
	}
}

// 2.5 — Concurrent Update calls do not race
func TestStatusSpinner_ConcurrentUpdate(t *testing.T) {
	var buf bytes.Buffer
	s := NewStatusSpinner()
	s.writer = &buf
	s.interval = 10 * time.Millisecond

	s.Start()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				s.Update("msg from goroutine")
			}
		}(i)
	}
	wg.Wait()

	s.Stop()
	// If we get here without a race detector complaint, the test passes
}

// Additional: double Start is safe
func TestStatusSpinner_DoubleStart(t *testing.T) {
	var buf bytes.Buffer
	s := NewStatusSpinner()
	s.writer = &buf
	s.interval = 10 * time.Millisecond

	s.Start()
	s.Start() // should be a no-op
	time.Sleep(30 * time.Millisecond)
	s.Stop()
}

// Additional: double Stop is safe
func TestStatusSpinner_DoubleStop(t *testing.T) {
	var buf bytes.Buffer
	s := NewStatusSpinner()
	s.writer = &buf
	s.interval = 10 * time.Millisecond

	s.Start()
	time.Sleep(30 * time.Millisecond)
	s.Stop()
	s.Stop() // should be a no-op, not panic
}
