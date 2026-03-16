## Context

The CLI commands `export`, `discover`, and `doctor`/`setup-browser` all perform long-running operations that can take seconds to hours. Currently:

- **export** (default mode): Complete silence between "Starting export..." and the summary table. The ~40 progress messages from the exporter are gated behind `--verbose`/`--debug`.
- **discover**: Inline `fmt.Printf` messages per conversation, but no animation.
- **doctor/setup-browser**: Lipgloss-styled pass/fail results, but no animation during the slow Chrome connection and Drive API checks.

The project already depends on `charmbracelet/huh` (which pulls in `bubbletea` and `bubbles` as transitive dependencies) and `charmbracelet/lipgloss`. The `bubbles/spinner` package is available without adding any new module dependencies.

## Goals / Non-Goals

### Goals
- Show an animated spinner with a changing status message during long-running operations so users know the process is alive
- Support graceful degradation: no spinner when `--verbose` is active (verbose already provides updates) or when stderr is not a TTY (piped/scripted use)
- Keep the implementation lightweight — no full Bubble Tea program, no alternate screen buffer, no terminal takeover
- Use `bubbles/spinner` frame definitions for consistent Charm ecosystem styling

### Non-Goals
- Full TUI with multi-line live-updating display (too heavy for the current CLI architecture)
- Progress bars with percentage (we don't always know the total message count upfront)
- Spinner on `list`, `status`, or `auth` commands (these are fast enough to not need it)
- Changing the exporter's `OnProgress` callback contract

## Decisions

### 1. Inline spinner via carriage return, not Bubble Tea

A full `bubbletea.NewProgram` would take over stdin/stdout handling and conflict with the existing `fmt.Printf` output pattern. Instead, the spinner uses a simple goroutine that writes to stderr via `\r` (carriage return) to overwrite a single line in place.

This approach:
- Works alongside existing `fmt.Println` output (which goes to stdout)
- Doesn't interfere with signal handlers or the cobra command lifecycle
- Degrades cleanly when stderr is not a TTY (just skips animation)

Satisfies constitution principle IV (Testability): the spinner goroutine is independent of any terminal state beyond a writer.

### 2. StatusSpinner type in internal/cli/spinner.go

A single type encapsulates all spinner behavior:

```go
type StatusSpinner struct {
    mu       sync.Mutex
    message  string
    writer   io.Writer       // defaults to os.Stderr
    frames   []string        // from bubbles/spinner
    interval time.Duration   // frame rotation speed
    done     chan struct{}
    active   bool
    style    lipgloss.Style  // for the spinner character
}

func NewStatusSpinner() *StatusSpinner
func (s *StatusSpinner) Start()
func (s *StatusSpinner) Update(msg string)
func (s *StatusSpinner) Stop()
```

- `Start()` launches the animation goroutine. Each tick: lock, read message, write `\r` + styled frame + message, unlock.
- `Update(msg)` sets the message under the lock. The next tick picks it up.
- `Stop()` signals the goroutine to exit, clears the spinner line with `\r` + spaces + `\r`, and sets `active = false`.
- The `writer` field enables testing by writing to a `bytes.Buffer` instead of stderr.

### 3. Frame source: bubbles/spinner.MiniDot

Use `spinner.MiniDot` frames (`⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏`) at 100ms intervals. These are compact, professional, and consistent with the Charm ecosystem. The `spinner` package is imported only for its frame set constants — the animation loop is our own.

### 4. Lipgloss styling

The spinner character is rendered in a dim style so it doesn't compete with the status message for attention. The conversation name within the message is rendered bold. This matches the styling approach already used in `doctor`/`setup-browser`.

### 5. TTY detection

Before starting the spinner, check `isatty(stderr)`. If stderr is not a TTY (piped, redirected, or running in CI), skip the spinner entirely. The existing `isTerminal()` function in `selfservice.go` already does this check — extract it to a shared location.

### 6. Verbose mode disables spinner

When `--verbose` or `--debug` is active, the spinner is not started. The existing `OnProgress` callback continues to print indented log lines via `fmt.Printf`. This avoids interleaving the spinner's `\r`-based line overwriting with verbose's newline-based output.

### 7. Integration pattern for export command

The export command's `OnProgress` callback is the integration point:

```go
// In runExport:
var spin *StatusSpinner
if !verbose && !debugMode && isTerminal() {
    spin = NewStatusSpinner()
    spin.Start()
}

cfg := &exporter.ExporterConfig{
    OnProgress: func(msg string) {
        if verbose || debugMode {
            fmt.Printf("  %s\n", msg)
        } else if spin != nil {
            spin.Update(msg)
        }
    },
    // ...
}

// After export completes:
if spin != nil {
    spin.Stop()
}
// Print summary table
```

### 8. Integration pattern for discover command

Replace the existing `fmt.Printf` calls with spinner updates in the member-fetching and user-profile-fetching phases. The per-conversation "Fetching members for X..." messages become spinner updates.

### 9. Integration pattern for doctor/setup-browser

For each slow check (Chrome connection, Drive API), start a spinner with "Checking...", then stop it and print the styled result. This creates a smooth: spinner animates → result appears pattern.

## Risks / Trade-offs

### Carriage return may not work in all terminals

Some terminals or terminal multiplexers may not handle `\r` correctly, leading to garbled output. Mitigation: the TTY detection skips the spinner in non-interactive contexts, and the `\r` approach is widely supported in modern terminals (iTerm2, Terminal.app, GNOME Terminal, Windows Terminal, VS Code).

### Spinner hides verbose detail by default

Users who run without `--verbose` now see the spinner instead of silence, but they still don't see the detailed progress messages. This is intentional — the spinner provides reassurance that something is happening, while verbose mode provides the full detail. The UX guidance is: if you want to see exactly what's happening, use `--verbose`.

### Thread safety with OnProgress callback

The exporter calls `OnProgress` from the export goroutine(s), and in parallel mode from multiple goroutines simultaneously. The `StatusSpinner.Update()` method uses a mutex to safely update the message. In parallel mode, the spinner will show whichever goroutine last called Update — messages may flicker between conversations. This is acceptable because the point is showing that the process is alive, not tracking every individual goroutine.
