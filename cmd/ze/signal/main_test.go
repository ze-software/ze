package signal

import (
	"net"
	"slices"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/core/env"
)

// VALIDATES: resolveHost returns flag value when set
// PREVENTS: env vars overriding explicit flag

func TestResolveHostFlag(t *testing.T) {
	got := resolveHost("10.0.0.1")
	if got != "10.0.0.1" {
		t.Errorf("resolveHost with flag: got %q, want %q", got, "10.0.0.1")
	}
}

// VALIDATES: resolveHost falls back to env var
// PREVENTS: env var not being read

func TestResolveHostEnv(t *testing.T) {
	t.Setenv("ze_ssh_host", "10.0.0.2")
	env.ResetCache()

	got := resolveHost("")
	if got != "10.0.0.2" {
		t.Errorf("resolveHost with env: got %q, want %q", got, "10.0.0.2")
	}
}

// VALIDATES: resolveHost returns default when no flag or env
// PREVENTS: empty host causing connection failure

func TestResolveHostDefault(t *testing.T) {
	t.Setenv("ze_ssh_host", "")
	t.Setenv("ze.ssh.host", "")
	env.ResetCache()

	got := resolveHost("")
	if got != defaultHost {
		t.Errorf("resolveHost default: got %q, want %q", got, defaultHost)
	}
}

// VALIDATES: resolvePort returns flag value when set
// PREVENTS: env vars overriding explicit flag

func TestResolvePortFlag(t *testing.T) {
	got := resolvePort("3333")
	if got != "3333" {
		t.Errorf("resolvePort with flag: got %q, want %q", got, "3333")
	}
}

// VALIDATES: resolvePort falls back to env var
// PREVENTS: env var not being read

func TestResolvePortEnv(t *testing.T) {
	t.Setenv("ze_ssh_port", "4444")
	env.ResetCache()

	got := resolvePort("")
	if got != "4444" {
		t.Errorf("resolvePort with env: got %q, want %q", got, "4444")
	}
}

// VALIDATES: resolvePort returns default when no flag or env
// PREVENTS: empty port causing connection failure

func TestResolvePortDefault(t *testing.T) {
	t.Setenv("ze_ssh_port", "")
	t.Setenv("ze.ssh.port", "")
	env.ResetCache()

	got := resolvePort("")
	if got != defaultPort {
		t.Errorf("resolvePort default: got %q, want %q", got, defaultPort)
	}
}

// VALIDATES: RunStatus detects running daemon by TCP dial
// PREVENTS: false negative when daemon is reachable

func TestRunStatusDaemonRunning(t *testing.T) {
	// Start a TCP listener to simulate a running daemon
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close() //nolint:errcheck // test cleanup

	_, port, _ := net.SplitHostPort(ln.Addr().String())

	code := RunStatus([]string{"--host", "127.0.0.1", "--port", port})
	if code != ExitSuccess {
		t.Errorf("expected ExitSuccess (0), got %d", code)
	}
}

// VALIDATES: RunStatus detects no daemon by TCP dial failure
// PREVENTS: false positive when daemon is not running

func TestRunStatusDaemonNotRunning(t *testing.T) {
	// Use a port that's definitely not listening
	code := RunStatus([]string{"--host", "127.0.0.1", "--port", "1"})
	if code != ExitNotRunning {
		t.Errorf("expected ExitNotRunning (1), got %d", code)
	}
}

// VALIDATES: Run requires at least one argument (command)
// PREVENTS: panic on empty args

func TestSignalCommandMissingArgs(t *testing.T) {
	code := Run([]string{})
	if code != ExitNotRunning {
		t.Errorf("expected ExitNotRunning, got %d", code)
	}
}

// VALIDATES: Run rejects unknown commands
// PREVENTS: silent failure on typos

func TestSignalCommandUnknown(t *testing.T) {
	// Set env to avoid actual connections
	t.Setenv("ze_ssh_host", "127.0.0.1")
	t.Setenv("ze_ssh_port", "1")
	env.ResetCache()

	code := Run([]string{"unknown"})
	if code != ExitNotRunning {
		t.Errorf("expected ExitNotRunning, got %d", code)
	}
}

// VALIDATES: Commands registry contains all expected signal subcommands
// PREVENTS: missing command after refactor

func TestCommandsRegistryComplete(t *testing.T) {
	expected := []string{"reload", "stop", "restart", "status", "quit"}
	names := Names()
	if len(names) != len(expected) {
		t.Fatalf("Commands count: got %d, want %d", len(names), len(expected))
	}
	for _, want := range expected {
		if !slices.Contains(names, want) {
			t.Errorf("missing command %q in Commands registry", want)
		}
	}
}

// VALIDATES: lookup returns correct ExecCommand for each signal name
// PREVENTS: wrong SSH exec string sent to daemon

func TestCommandExecMapping(t *testing.T) {
	tests := []struct {
		name string
		exec string
	}{
		{"reload", "daemon reload"},
		{"stop", "stop"},
		{"restart", "restart"},
		{"status", "daemon status"},
		{"quit", "daemon quit"},
	}
	for _, tt := range tests {
		cmd := lookup(tt.name)
		if cmd == nil {
			t.Errorf("lookup(%q) returned nil", tt.name)
			continue
		}
		if cmd.ExecCommand != tt.exec {
			t.Errorf("lookup(%q).ExecCommand = %q, want %q", tt.name, cmd.ExecCommand, tt.exec)
		}
	}
}

// VALIDATES: Commands() returns a defensive copy
// PREVENTS: callers mutating the internal registry

func TestCommandsReturnsCopy(t *testing.T) {
	cmds := Commands()
	cmds[0].Name = "MUTATED"
	if lookup("MUTATED") != nil {
		t.Error("Commands() returned a reference to the internal slice, not a copy")
	}
}

// VALIDATES: lookup returns nil for unknown command
// PREVENTS: panic on invalid input

func TestLookupUnknown(t *testing.T) {
	if cmd := lookup("nonexistent"); cmd != nil {
		t.Errorf("lookup(nonexistent) = %v, want nil", cmd)
	}
}

// VALIDATES: AC-8 -- env var with uppercase ZE_SSH_HOST works via env.Get normalization
// PREVENTS: only lowercase or dot notation being recognized

func TestResolveHostUppercaseEnv(t *testing.T) {
	t.Setenv("ze_ssh_host", "")
	t.Setenv("ze.ssh.host", "")
	t.Setenv("ZE_SSH_HOST", "10.0.0.5")
	env.ResetCache()

	got := resolveHost("")
	if got != "10.0.0.5" {
		t.Errorf("resolveHost uppercase env: got %q, want %q", got, "10.0.0.5")
	}
}

// VALIDATES: AC-8 -- env var with uppercase ZE_SSH_PORT works via env.Get normalization
// PREVENTS: only lowercase or dot notation being recognized

func TestResolvePortUppercaseEnv(t *testing.T) {
	t.Setenv("ze_ssh_port", "")
	t.Setenv("ze.ssh.port", "")
	t.Setenv("ZE_SSH_PORT", "2223")
	env.ResetCache()

	got := resolvePort("")
	if got != "2223" {
		t.Errorf("resolvePort uppercase env: got %q, want %q", got, "2223")
	}
}
