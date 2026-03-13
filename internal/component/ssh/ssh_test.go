package ssh

import (
	"context"
	"net"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/cli"
)

// VALIDATES: AC-4 — SSH server created with config values.
// PREVENTS: server ignoring configured address, timeouts, or user list.

func TestNewServer(t *testing.T) {
	cfg := Config{
		Listen:      "127.0.0.1:2222",
		HostKeyPath: t.TempDir() + "/test_host_key",
		IdleTimeout: 300,
		MaxSessions: 4,
		Users: []UserConfig{
			{Name: "admin", Hash: "$2a$10$fake"},
		},
	}

	srv, err := NewServer(cfg)
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1:2222", srv.Address())
	assert.Equal(t, 4, srv.MaxSessions())
	assert.Equal(t, 1, len(srv.Users()))
}

func TestNewServerDefaults(t *testing.T) {
	cfg := Config{
		HostKeyPath: t.TempDir() + "/test_host_key",
	}

	srv, err := NewServer(cfg)
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1:2222", srv.Address(), "default listen address")
	assert.Equal(t, 8, srv.MaxSessions(), "default max sessions")
}

func TestNewServerNoUsers(t *testing.T) {
	cfg := Config{
		Listen:      "127.0.0.1:2222",
		HostKeyPath: t.TempDir() + "/test_host_key",
	}

	srv, err := NewServer(cfg)
	require.NoError(t, err)
	assert.Empty(t, srv.Users(), "no users configured")
}

func TestServerName(t *testing.T) {
	cfg := Config{
		HostKeyPath: t.TempDir() + "/test_host_key",
	}

	srv, err := NewServer(cfg)
	require.NoError(t, err)
	assert.Equal(t, "ssh", srv.Name(), "subsystem name")
}

// VALIDATES: host-key defaults to ConfigDir/ssh_host_ed25519_key when omitted.
// PREVENTS: server failing to start because no host key path was configured.
func TestNewServerDefaultHostKeyFromConfigDir(t *testing.T) {
	cfg := Config{
		Listen:    "127.0.0.1:2222",
		ConfigDir: "/opt/ze/etc",
	}

	srv, err := NewServer(cfg)
	require.NoError(t, err)
	assert.Equal(t, "/opt/ze/etc/ssh_host_ed25519_key", srv.config.HostKeyPath)
}

// VALIDATES: NewServer without host-key or ConfigDir falls back to binary-relative resolution.
// PREVENTS: empty host key path reaching Wish.
func TestNewServerNoHostKeyNoConfigDir(t *testing.T) {
	cfg := Config{
		Listen: "127.0.0.1:2222",
	}

	// The result depends on os.Executable() — test binary location varies.
	// We verify the contract: either a valid HostKeyPath is resolved, or a clear error.
	srv, err := NewServer(cfg)
	if err != nil {
		assert.Contains(t, err.Error(), "host-key path cannot be resolved")
		assert.Nil(t, srv)
		return
	}
	assert.NotEmpty(t, srv.config.HostKeyPath)
	assert.Contains(t, srv.config.HostKeyPath, "ssh_host_ed25519_key")
}

// VALIDATES: explicit host-key is preserved.
// PREVENTS: default overwriting an explicit config value.
func TestNewServerExplicitHostKey(t *testing.T) {
	cfg := Config{
		Listen:      "127.0.0.1:2222",
		HostKeyPath: "/custom/path/host_key",
		ConfigDir:   "/opt/ze/etc",
	}

	srv, err := NewServer(cfg)
	require.NoError(t, err)
	assert.Equal(t, "/custom/path/host_key", srv.config.HostKeyPath)
}

// VALIDATES: Bug 3 — double Start returns error instead of leaking first server.
// PREVENTS: orphaned listeners from calling Start() twice.
func TestServerDoubleStartError(t *testing.T) {
	cfg := Config{
		Listen:      "127.0.0.1:0",
		HostKeyPath: t.TempDir() + "/test_host_key",
	}

	srv, err := NewServer(cfg)
	require.NoError(t, err)

	// First start should succeed.
	err = srv.Start(context.Background(), nil, nil)
	require.NoError(t, err)

	// Give the Serve goroutine time to register the listener with the server
	// (avoids a race in the Charm SSH library between Serve and Shutdown).
	time.Sleep(50 * time.Millisecond)

	// Second start should fail.
	err = srv.Start(context.Background(), nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already started")

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	require.NoError(t, srv.Stop(stopCtx))
}

// VALIDATES: Bug 2 — Start fails synchronously when port is in use.
// PREVENTS: silent bind failures that Start() returns nil for.
func TestServerStartBindFailure(t *testing.T) {
	// Occupy a port with a raw listener first to get a known-busy port.
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close() //nolint:errcheck // test cleanup

	busyAddr := ln.Addr().String()

	// Try to start SSH server on the occupied port.
	cfg := Config{
		Listen:      busyAddr,
		HostKeyPath: t.TempDir() + "/test_host_key",
	}
	srv, err := NewServer(cfg)
	require.NoError(t, err)

	err = srv.Start(context.Background(), nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bind SSH server")
}

// VALIDATES: Bug 4 — max-sessions is tracked via activeSessions counter.
// PREVENTS: unlimited concurrent sessions ignoring the configured limit.
func TestServerActiveSessionsCounter(t *testing.T) {
	cfg := Config{
		HostKeyPath: t.TempDir() + "/test_host_key",
		MaxSessions: 2,
	}

	srv, err := NewServer(cfg)
	require.NoError(t, err)

	// Initially zero active sessions.
	assert.Equal(t, int32(0), srv.ActiveSessions())

	// Simulate session counting (directly via atomic, since middleware is tested via integration).
	srv.activeSessions.Add(1)
	assert.Equal(t, int32(1), srv.ActiveSessions())
	srv.activeSessions.Add(1)
	assert.Equal(t, int32(2), srv.ActiveSessions())
	srv.activeSessions.Add(-1)
	assert.Equal(t, int32(1), srv.ActiveSessions())
}

// VALIDATES: AC-5 — SSH session uses unified cli.Model in command mode.
// PREVENTS: SSH sessions missing features available in other TUI entry points.
func TestSSHUsesUnifiedModel(t *testing.T) {
	// Create a server with an executor factory.
	executorCalled := false
	factory := func(username string) CommandExecutor {
		return func(input string) (string, error) {
			executorCalled = true
			return "result:" + input, nil
		}
	}

	cfg := Config{
		HostKeyPath: t.TempDir() + "/test_host_key",
	}
	srv, err := NewServer(cfg)
	require.NoError(t, err)
	srv.SetExecutorFactory(factory)

	// createSessionModel is the extracted function that teaHandler uses.
	model := srv.createSessionModel("testuser")

	// Verify it starts in ModeCommand (command-only, no editor).
	assert.Equal(t, cli.ModeCommand, model.Mode(), "SSH model should start in command mode")

	// Verify the executor is wired: send a command through Update.
	// Bubble Tea commands are async — Update returns a tea.Cmd that must be
	// executed manually in tests, then the resulting message fed back through Update.
	model.SetInput("test-command")
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd, "Enter should return a command for async execution")

	// Execute the command and feed the result back.
	msg := cmd()
	updated, _ = updated.Update(msg)
	m, ok := updated.(cli.Model)
	require.True(t, ok, "Update should return cli.Model")
	assert.True(t, executorCalled, "executor should be called via async command")
	assert.Contains(t, m.ViewportContent(), "result:test-command")
}
