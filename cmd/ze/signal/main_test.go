package signal

import (
	"net"
	"os"
	"testing"
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
	code := Run([]string{"unknown"})
	if code != ExitNotRunning {
		t.Errorf("expected ExitNotRunning, got %d", code)
	}
}

// VALIDATES: env var with dot notation (ze.ssh.host) works
// PREVENTS: only underscore variant being recognized

func TestResolveHostDotEnv(t *testing.T) {
	// Clear underscore variant, set dot variant
	t.Setenv("ze_ssh_host", "")
	if err := os.Setenv("ze.ssh.host", "10.0.0.3"); err != nil {
		t.Skip("cannot set env with dots on this platform")
	}
	defer os.Unsetenv("ze.ssh.host") //nolint:errcheck // test cleanup

	got := resolveHost("")
	if got != "10.0.0.3" {
		t.Errorf("resolveHost dot env: got %q, want %q", got, "10.0.0.3")
	}
}
