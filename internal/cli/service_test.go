package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// 5.1 — generateWrapper contains expected content
func TestGenerateWrapper(t *testing.T) {
	wrapper := generateWrapper("/usr/local/bin/get-out", "/var/log/get-out.log", "/home/user/.get-out/.env")

	checks := []struct {
		name     string
		contains string
	}{
		{"shebang", "#!/bin/bash"},
		{"binary path", "'/usr/local/bin/get-out'"},
		{"export command", "export --sync --parallel 5"},
		{"env file", "/home/user/.get-out/.env"},
		{"log file", "/var/log/get-out.log"},
		{"log rotation", "MAX_LOG_BYTES"},
		{"timestamp", "date '+%Y-%m-%d %H:%M:%S'"},
		{"exit code", "Exit code: $RC"},
	}

	for _, c := range checks {
		if !strings.Contains(wrapper, c.contains) {
			t.Errorf("wrapper missing %s (%q)", c.name, c.contains)
		}
	}
}

func TestGenerateWrapper_QuotesBinaryPath(t *testing.T) {
	// Binary path with spaces should be single-quoted
	wrapper := generateWrapper("/path/with spaces/get-out", "/log.log", "/env")
	if !strings.Contains(wrapper, "'/path/with spaces/get-out'") {
		t.Error("wrapper should single-quote binary path with spaces")
	}

	// Binary path with single quote should be escaped
	wrapper2 := generateWrapper("/it's/get-out", "/log.log", "/env")
	if !strings.Contains(wrapper2, "'\\''") {
		t.Error("wrapper should escape single quotes in binary path")
	}
}

// 5.2 — generatePlist contains expected content
func TestGeneratePlist(t *testing.T) {
	plist := generatePlist("/home/user/.local/bin/get-out-wrapper.sh", "/home/user/Library/Logs/get-out.log", "/home/user", "/usr/local/bin/get-out")

	checks := []struct {
		name     string
		contains string
	}{
		{"xml header", "<?xml version"},
		{"label", serviceLabel},
		{"wrapper path", "/home/user/.local/bin/get-out-wrapper.sh"},
		{"start interval", "<integer>3600</integer>"},
		{"run at load", "<true/>"},
		{"log path", "/home/user/Library/Logs/get-out.log"},
		{"homebrew path", "/opt/homebrew/bin"},
		{"home env", "/home/user"},
	}

	for _, c := range checks {
		if !strings.Contains(plist, c.contains) {
			t.Errorf("plist missing %s (%q)", c.name, c.contains)
		}
	}
}

// 5.3 — generateSystemdService contains expected content
func TestGenerateSystemdService(t *testing.T) {
	service := generateSystemdService("/home/user/.local/bin/get-out-wrapper.sh", "/home/user")

	checks := []struct {
		name     string
		contains string
	}{
		{"type oneshot", "Type=oneshot"},
		{"exec start", "ExecStart=/bin/bash /home/user/.local/bin/get-out-wrapper.sh"},
		{"home env", "Environment=HOME=/home/user"},
		{"description", "get-out"},
		{"install section", "[Install]"},
	}

	for _, c := range checks {
		if !strings.Contains(service, c.contains) {
			t.Errorf("service unit missing %s (%q)", c.name, c.contains)
		}
	}
}

// 5.4 — generateSystemdTimer contains expected content
func TestGenerateSystemdTimer(t *testing.T) {
	timer := generateSystemdTimer()

	checks := []struct {
		name     string
		contains string
	}{
		{"hourly", "OnCalendar=hourly"},
		{"persistent", "Persistent=true"},
		{"randomized delay", "RandomizedDelaySec=120"},
		{"timer target", "WantedBy=timers.target"},
	}

	for _, c := range checks {
		if !strings.Contains(timer, c.contains) {
			t.Errorf("timer unit missing %s (%q)", c.name, c.contains)
		}
	}
}

// 5.5 — isServiceInstalled returns false when files don't exist
func TestIsServiceInstalled_NotInstalled(t *testing.T) {
	// Save and override HOME to a temp dir where no service files exist
	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// Create the directories the function checks (so Stat doesn't fail on the parent)
	switch runtime.GOOS {
	case "darwin":
		os.MkdirAll(filepath.Join(tmpHome, "Library", "LaunchAgents"), 0755)
	case "linux":
		os.MkdirAll(filepath.Join(tmpHome, ".config", "systemd", "user"), 0755)
	}

	if isServiceInstalled() {
		t.Error("isServiceInstalled() = true, want false (no service files)")
	}
}
