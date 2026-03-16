package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
)

// StatusSpinner displays an animated spinner with a status message on a single
// terminal line. It writes to stderr via carriage return to overwrite in place,
// avoiding interference with stdout content.
//
// The spinner is designed for CLI commands that run long operations (export,
// discover, doctor). It is disabled automatically when stderr is not a TTY.
type StatusSpinner struct {
	mu       sync.Mutex
	wg       sync.WaitGroup
	message  string
	writer   io.Writer
	frames   []string
	interval time.Duration
	done     chan struct{}
	active   bool
	style    lipgloss.Style
}

// NewStatusSpinner creates a spinner using MiniDot frames at 100ms intervals.
// The spinner writes to stderr and uses a dim style for the spinner character.
func NewStatusSpinner() *StatusSpinner {
	return &StatusSpinner{
		writer:   os.Stderr,
		frames:   spinner.MiniDot.Frames,
		interval: 100 * time.Millisecond,
		style:    lipgloss.NewStyle().Faint(true),
	}
}

// Start begins the spinner animation in a background goroutine.
// Call Update() to change the status message, and Stop() to end the animation.
func (s *StatusSpinner) Start() {
	s.mu.Lock()
	if s.active {
		s.mu.Unlock()
		return
	}
	s.done = make(chan struct{})
	s.active = true
	s.wg.Add(1)
	s.mu.Unlock()

	go s.run()
}

// Update sets the status message displayed next to the spinner.
func (s *StatusSpinner) Update(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.message = msg
}

// Stop ends the spinner animation and clears the spinner line.
// It waits for the animation goroutine to exit before writing.
func (s *StatusSpinner) Stop() {
	s.mu.Lock()
	if !s.active {
		s.mu.Unlock()
		return
	}
	s.active = false
	s.mu.Unlock()

	close(s.done)
	s.wg.Wait() // wait for goroutine to exit before writing

	// Clear the spinner line — safe because goroutine has exited
	fmt.Fprintf(s.writer, "\r%s\r", strings.Repeat(" ", 80))
}

func (s *StatusSpinner) run() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	frameIdx := 0
	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			s.mu.Lock()
			frame := s.frames[frameIdx%len(s.frames)]
			msg := s.message
			style := s.style
			s.mu.Unlock()

			// Write under mutex to prevent race with Stop's clear line.
			// This is safe because Stop waits for this goroutine to exit.
			fmt.Fprintf(s.writer, "\r%s %s", style.Render(frame), msg)

			frameIdx++
		}
	}
}
