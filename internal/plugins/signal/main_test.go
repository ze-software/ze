package signal

import (
	"net"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/env"
)

// resetResolveEnv removes all spellings of the requested exact keys. Call it
// before t.Setenv: cleanup restores inherited values before resetting the cache.
func resetResolveEnv(t *testing.T, keys ...string) {
	t.Helper()
	t.Cleanup(env.ResetCache)
	if len(keys) == 0 {
		env.ResetCache()
		return
	}
	environ := os.Environ()
	kept := env.Without(environ, keys...)
	remaining := len(kept)
	// Reverse removal makes LIFO cleanup restore same-kind alias precedence.
	for _, entry := range slices.Backward(environ) {
		if remaining > 0 {
			if entry == kept[remaining-1] {
				remaining--
				continue
			}
		}
		name, value, _ := strings.Cut(entry, "=")
		t.Setenv(name, value)
		if err := os.Unsetenv(name); err != nil {
			t.Fatalf("unset %s: %v", name, err)
		}
	}
	env.ResetCache()
}

// TestResetResolveEnvRestoresAliases verifies exact-key isolation and that
// cleanup restores inherited spellings before invalidating a populated cache.
func TestResetResolveEnvRestoresAliases(t *testing.T) {
	resetResolveEnv(t, "ze.ssh.host")
	t.Setenv("ze.ssh.host", "10.0.0.7")
	t.Setenv("Ze.SsH_hOsT", "10.0.0.9")
	t.Setenv("ZE_SSH_HOST", "10.0.0.8")
	t.Setenv("Ze.SsH_hOsT.extra", "untouched")
	if got := resolveHost(""); got != "10.0.0.9" {
		t.Fatalf("inherited host: got %q, want 10.0.0.9", got)
	}

	t.Run("isolated", func(t *testing.T) {
		resetResolveEnv(t, "ze.ssh.host")
		for _, name := range []string{"ze.ssh.host", "Ze.SsH_hOsT", "ZE_SSH_HOST"} {
			if _, present := os.LookupEnv(name); present {
				t.Errorf("%s must be absent", name)
			}
		}
		if value, _ := os.LookupEnv("Ze.SsH_hOsT.extra"); value != "untouched" {
			t.Errorf("neighboring key: got %q, want untouched", value)
		}
		t.Setenv("ZE_SSH_HOST", "10.0.0.5")
		if got := resolveHost(""); got != "10.0.0.5" {
			t.Errorf("isolated host: got %q, want 10.0.0.5", got)
		}
	})

	for name, want := range map[string]string{
		"ze.ssh.host": "10.0.0.7",
		"Ze.SsH_hOsT": "10.0.0.9",
		"ZE_SSH_HOST": "10.0.0.8",
	} {
		if value, _ := os.LookupEnv(name); value != want {
			t.Errorf("restored %s: got %q, want %q", name, value, want)
		}
	}
	if got := resolveHost(""); got != "10.0.0.9" {
		t.Errorf("restored cached host: got %q, want 10.0.0.9", got)
	}
}

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
	resetResolveEnv(t, "ze.ssh.host")
	t.Setenv("ze_ssh_host", "10.0.0.2")

	got := resolveHost("")
	if got != "10.0.0.2" {
		t.Errorf("resolveHost with env: got %q, want %q", got, "10.0.0.2")
	}
}

// VALIDATES: resolveHost returns default when no flag or env
// PREVENTS: empty host causing connection failure

func TestResolveHostDefault(t *testing.T) {
	resetResolveEnv(t, "ze.ssh.host")

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
	resetResolveEnv(t, "ze.ssh.port")
	t.Setenv("ze_ssh_port", "4444")

	got := resolvePort("")
	if got != "4444" {
		t.Errorf("resolvePort with env: got %q, want %q", got, "4444")
	}
}

// VALIDATES: resolvePort returns default when no flag or env
// PREVENTS: empty port causing connection failure

func TestResolvePortDefault(t *testing.T) {
	resetResolveEnv(t, "ze.ssh.port")

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
	resetResolveEnv(t, "ze.ssh.host", "ze.ssh.port")
	t.Setenv("ze_ssh_host", "127.0.0.1")
	t.Setenv("ze_ssh_port", "1")

	code := Run([]string{"unknown"})
	if code != ExitNotRunning {
		t.Errorf("expected ExitNotRunning, got %d", code)
	}
}

// VALIDATES: Commands registry contains all expected signal subcommands
// PREVENTS: missing command after refactor

func TestCommandsRegistryComplete(t *testing.T) {
	expected := []string{"reload", "stop", "restart", "reboot", "status", "quit"}
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
		{"reload", "request reload"},
		{"stop", "stop"},
		{"restart", "restart"},
		{"reboot", "reboot"},
		{"status", "show status"},
		{"quit", "request halt"},
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

// sshLifecycleVerbs are the exec strings the SSH server intercepts in
// execMiddleware (internal/component/ssh/ssh.go) BEFORE the command dispatcher
// sees them. The dispatcher deliberately registers no key for these, so they
// are the one legitimate exception to the verb-first rule below.
var sshLifecycleVerbs = map[string]bool{"stop": true, "restart": true, "reboot": true}

// VALIDATES: every ExecCommand is a surface the daemon actually answers -- an
// SSH lifecycle verb, or a command whose first token is a registered CLI verb
// (command.Verbs, the canonical vocabulary the grammar gate derives from).
// PREVENTS: the table drifting back to a noun-first spelling the verb-first
// migration removed from the command tree ("daemon reload", "daemon status",
// "daemon quit"), which the dispatcher answers with ErrUnknownCommand while
// the exec-mapping table above still passes.

func TestCommandExecReachesRealSurface(t *testing.T) {
	for _, c := range Commands() {
		if sshLifecycleVerbs[c.ExecCommand] {
			continue
		}
		verb, _, ok := strings.Cut(c.ExecCommand, " ")
		if !ok {
			t.Errorf("signal %q: ExecCommand %q is a single token and is not an SSH lifecycle verb; it cannot reach the dispatcher", c.Name, c.ExecCommand)
			continue
		}
		if !command.IsVerb(verb) {
			t.Errorf("signal %q: ExecCommand %q starts with %q, which is not a registered CLI verb (internal/component/command.Verbs); the dispatcher has no such key", c.Name, c.ExecCommand, verb)
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
	resetResolveEnv(t, "ze.ssh.host")
	t.Setenv("ZE_SSH_HOST", "10.0.0.5")

	got := resolveHost("")
	if got != "10.0.0.5" {
		t.Errorf("resolveHost uppercase env: got %q, want %q", got, "10.0.0.5")
	}
}

// VALIDATES: AC-8 -- env var with uppercase ZE_SSH_PORT works via env.Get normalization
// PREVENTS: only lowercase or dot notation being recognized

func TestResolvePortUppercaseEnv(t *testing.T) {
	resetResolveEnv(t, "ze.ssh.port")
	t.Setenv("ZE_SSH_PORT", "2223")

	got := resolvePort("")
	if got != "2223" {
		t.Errorf("resolvePort uppercase env: got %q, want %q", got, "2223")
	}
}
