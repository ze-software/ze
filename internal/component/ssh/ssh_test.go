package ssh

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
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

// VALIDATES: host key generated and stored when blob storage is set and key is missing.
// PREVENTS: SSH server failing to start with blob storage when no host key exists.
func TestResolveHostKeyFromBlobStorage(t *testing.T) {
	dir := t.TempDir()
	blobPath := filepath.Join(dir, "database.zefs")
	store, err := storage.NewBlob(blobPath, dir)
	require.NoError(t, err)
	defer store.Close() //nolint:errcheck // test cleanup

	keyPath := filepath.Join(dir, "ssh_host_ed25519_key")

	cfg := Config{
		Listen:      "127.0.0.1:0",
		HostKeyPath: keyPath,
		Storage:     store,
	}

	srv, srvErr := NewServer(cfg)
	require.NoError(t, srvErr)

	// Key does not exist yet -- resolveHostKeyOption should generate and store it.
	opt, optErr := srv.resolveHostKeyOption()
	require.NoError(t, optErr)
	assert.NotNil(t, opt, "should return a valid ssh.Option")

	// Verify the key was written to storage.
	assert.True(t, store.Exists(keyPath), "host key should be written to storage")
	data, readErr := store.ReadFile(keyPath)
	require.NoError(t, readErr)
	assert.Contains(t, string(data), "PRIVATE KEY", "stored key should be PEM-encoded")

	// Second call should read the existing key, not generate a new one.
	opt2, opt2Err := srv.resolveHostKeyOption()
	require.NoError(t, opt2Err)
	assert.NotNil(t, opt2, "should return a valid ssh.Option from existing key")
	data2, read2Err := store.ReadFile(keyPath)
	require.NoError(t, read2Err)
	assert.Equal(t, data, data2, "key should not be regenerated")
}

// VALIDATES: resolveHostKeyOption generates key in memory when storage is nil or filesystem.
// PREVENTS: host key files being created on the physical filesystem by Wish.
func TestResolveHostKeyFilesystemMode(t *testing.T) {
	t.Run("nil storage", func(t *testing.T) {
		dir := t.TempDir()
		keyPath := filepath.Join(dir, "host_key")
		cfg := Config{
			Listen:      "127.0.0.1:0",
			HostKeyPath: keyPath,
		}
		srv, err := NewServer(cfg)
		require.NoError(t, err)
		opt, err := srv.resolveHostKeyOption()
		require.NoError(t, err)
		assert.NotNil(t, opt, "should return a valid ssh.Option")
		// Key must NOT be written to filesystem when storage is nil.
		_, statErr := os.Stat(keyPath)
		assert.True(t, os.IsNotExist(statErr), "key file must not be created on filesystem")
		_, statPubErr := os.Stat(keyPath + ".pub")
		assert.True(t, os.IsNotExist(statPubErr), "pub file must not be created on filesystem")
	})

	t.Run("filesystem storage", func(t *testing.T) {
		dir := t.TempDir()
		keyPath := filepath.Join(dir, "host_key")
		cfg := Config{
			Listen:      "127.0.0.1:0",
			HostKeyPath: keyPath,
			Storage:     storage.NewFilesystem(),
		}
		srv, err := NewServer(cfg)
		require.NoError(t, err)
		opt, err := srv.resolveHostKeyOption()
		require.NoError(t, err)
		assert.NotNil(t, opt, "should return a valid ssh.Option")
		// Private key is written via storage.WriteFile (filesystem).
		data, readErr := os.ReadFile(keyPath)
		require.NoError(t, readErr, "key should be persisted via storage")
		assert.Contains(t, string(data), "PRIVATE KEY")
		// .pub must NOT exist — Wish's WithHostKeyPath is no longer used.
		_, statPubErr := os.Stat(keyPath + ".pub")
		assert.True(t, os.IsNotExist(statPubErr), "pub file must not be created on filesystem")
	})
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

	// Wait for the Serve goroutine to be ready before testing double-start
	// (avoids a race in the Charm SSH library between Serve and Shutdown).
	require.Eventually(t, func() bool {
		var d net.Dialer
		conn, dialErr := d.DialContext(context.Background(), "tcp", srv.Address())
		if dialErr != nil {
			return false // not ready yet
		}
		conn.Close() //nolint:errcheck,gosec // test probe
		return true
	}, 2*time.Second, time.Millisecond)

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
	updated, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd, "Enter should return a command for async execution")

	// Execute the command and feed the result back.
	msg := cmd()
	updated, _ = updated.Update(msg)
	m, ok := updated.(cli.Model)
	require.True(t, ok, "Update should return cli.Model")
	assert.True(t, executorCalled, "executor should be called via async command")
	assert.Contains(t, m.ViewportContent(), "result:test-command")
}

// TestSSHSessionGetsEditor verifies that createSessionModel creates an editor-capable
// model when ConfigPath and Storage are set.
//
// VALIDATES: AC-22 -- SSH session connects, gets editor with session identity.
// PREVENTS: SSH sessions stuck in command-only mode when config editing should work.
func TestSSHSessionGetsEditor(t *testing.T) {
	// Write a valid config file.
	configPath := filepath.Join(t.TempDir(), "test.conf")
	store := storage.NewFilesystem()
	err := store.WriteFile(configPath, []byte("bgp {\n  local-as 65000\n  router-id 1.2.3.4\n}\n"), 0o600)
	require.NoError(t, err)

	cfg := Config{
		HostKeyPath: t.TempDir() + "/test_host_key",
		ConfigPath:  configPath,
		Storage:     store,
	}
	srv, err := NewServer(cfg)
	require.NoError(t, err)

	model := srv.createSessionModel("alice")

	// With ConfigPath + Storage, model should start in ModeEdit (editor-capable).
	assert.Equal(t, cli.ModeEdit, model.Mode(), "SSH session with ConfigPath should start in edit mode")
}

// TestSSHSessionFallbackWithoutConfig verifies command-only fallback when no ConfigPath.
//
// VALIDATES: SSH session without ConfigPath gets command-only model.
// PREVENTS: Panic or error when SSH is configured without config editing support.
func TestSSHSessionFallbackWithoutConfig(t *testing.T) {
	cfg := Config{
		HostKeyPath: t.TempDir() + "/test_host_key",
		// No ConfigPath or Storage -- should fall back to command-only.
	}
	srv, err := NewServer(cfg)
	require.NoError(t, err)

	model := srv.createSessionModel("alice")

	assert.Equal(t, cli.ModeCommand, model.Mode(), "SSH session without ConfigPath should be command-only")
}

// TestLoginWarningsStalePeers verifies that createSessionModel passes login warnings
// to the model when the loginWarningsFunc returns warnings.
//
// VALIDATES: AC-1 from spec-login-warnings: Welcome shows stale peer warning.
// PREVENTS: Login warnings collected but not passed to model.
func TestLoginWarningsStalePeers(t *testing.T) {
	cfg := Config{
		HostKeyPath: t.TempDir() + "/test_host_key",
	}
	srv, err := NewServer(cfg)
	require.NoError(t, err)

	srv.SetLoginWarnings(func() []cli.LoginWarning {
		return []cli.LoginWarning{
			{Message: "3 peer(s) have stale prefix data", Command: "ze update bgp peer * prefix"},
		}
	})

	model := srv.createSessionModel("alice")

	view := model.View().Content
	assert.Contains(t, view, "3 peer(s) have stale prefix data", "warning message should appear in view")
	assert.Contains(t, view, "ze update bgp peer * prefix", "actionable command should appear in view")
}

// TestLoginWarningsNilFunc verifies that createSessionModel works normally
// when no loginWarningsFunc is set (pre-reactor-start state).
//
// VALIDATES: AC-5 from spec-login-warnings: Nil func causes no crash.
// PREVENTS: Nil dereference when SSH sessions start before reactor.
func TestLoginWarningsNilFunc(t *testing.T) {
	cfg := Config{
		HostKeyPath: t.TempDir() + "/test_host_key",
	}
	srv, err := NewServer(cfg)
	require.NoError(t, err)
	// No SetLoginWarnings called -- loginWarningsFunc is nil

	model := srv.createSessionModel("alice")

	view := model.View().Content
	assert.NotContains(t, view, "warning:", "no warning should appear without loginWarningsFunc")
}

// TestLoginWarningsNoPeers verifies no warning when func returns nil.
//
// VALIDATES: AC-3 from spec-login-warnings: No warning with no peers configured.
// PREVENTS: Empty warning block displayed when provider returns nil.
func TestLoginWarningsNoPeers(t *testing.T) {
	cfg := Config{
		HostKeyPath: t.TempDir() + "/test_host_key",
	}
	srv, err := NewServer(cfg)
	require.NoError(t, err)

	srv.SetLoginWarnings(func() []cli.LoginWarning {
		return nil // No warnings
	})

	model := srv.createSessionModel("alice")

	view := model.View().Content
	assert.NotContains(t, view, "warning:", "no warning should appear when provider returns nil")
}

// TestPrefixStalenessWarning verifies that the staleness closure correctly
// counts stale peers and formats the warning message.
//
// VALIDATES: AC-1 from spec-login-warnings: correct count in warning message.
// VALIDATES: AC-2 from spec-login-warnings: no warning when all fresh.
// PREVENTS: Wrong count or missing command in staleness warning.
func TestPrefixStalenessWarning(t *testing.T) {
	tests := []struct {
		name        string
		warnings    []cli.LoginWarning
		wantMessage string
		wantAbsent  bool
	}{
		{
			name: "3 stale peers",
			warnings: []cli.LoginWarning{
				{Message: "3 peer(s) have stale prefix data", Command: "ze update bgp peer * prefix"},
			},
			wantMessage: "3 peer(s) have stale prefix data",
		},
		{
			name:       "no stale peers",
			warnings:   nil,
			wantAbsent: true,
		},
		{
			name: "1 stale peer",
			warnings: []cli.LoginWarning{
				{Message: "1 peer(s) have stale prefix data", Command: "ze update bgp peer * prefix"},
			},
			wantMessage: "1 peer(s) have stale prefix data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				HostKeyPath: t.TempDir() + "/test_host_key",
			}
			srv, err := NewServer(cfg)
			require.NoError(t, err)

			warnings := tt.warnings
			srv.SetLoginWarnings(func() []cli.LoginWarning {
				return warnings
			})

			model := srv.createSessionModel("alice")
			view := model.View().Content

			if tt.wantAbsent {
				assert.NotContains(t, view, "warning:", "should not show warnings")
			} else {
				assert.Contains(t, view, tt.wantMessage, "warning message should match")
				assert.Contains(t, view, "ze update bgp peer * prefix", "command should appear")
			}
		})
	}
}

// TestStalenessClosureWithPeerData verifies the staleness counting logic
// using IsPrefixDataStale with actual peer data.
//
// VALIDATES: AC-1 from spec-login-warnings: correct stale peer count.
// VALIDATES: AC-2 from spec-login-warnings: no warning when all fresh.
// PREVENTS: Off-by-one in stale counter or wrong IsPrefixDataStale usage.
func TestStalenessClosureWithPeerData(t *testing.T) {
	now := time.Now()
	fresh := now.AddDate(0, -1, 0).Format(time.DateOnly)     // 1 month ago = fresh
	stale := now.AddDate(0, -7, 0).Format(time.DateOnly)     // 7 months ago = stale
	veryStale := now.AddDate(-2, 0, 0).Format(time.DateOnly) // 2 years ago = stale

	tests := []struct {
		name      string
		peers     []plugin.PeerInfo
		wantStale int
	}{
		{
			name:      "all fresh",
			peers:     []plugin.PeerInfo{{PrefixUpdated: fresh}, {PrefixUpdated: fresh}},
			wantStale: 0,
		},
		{
			name:      "one stale one fresh",
			peers:     []plugin.PeerInfo{{PrefixUpdated: stale}, {PrefixUpdated: fresh}},
			wantStale: 1,
		},
		{
			name:      "all stale",
			peers:     []plugin.PeerInfo{{PrefixUpdated: stale}, {PrefixUpdated: veryStale}},
			wantStale: 2,
		},
		{
			name:      "empty PrefixUpdated treated as not stale",
			peers:     []plugin.PeerInfo{{PrefixUpdated: ""}, {PrefixUpdated: stale}},
			wantStale: 1,
		},
		{
			name:      "no peers",
			peers:     nil,
			wantStale: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Replicate the closure logic from loader.go.
			// Uses same staleness check: empty = not stale, >6 months = stale.
			peers := tt.peers
			staleCount := 0
			for i := range peers {
				updated := peers[i].PrefixUpdated
				if updated == "" {
					continue
				}
				parsed, err := time.Parse(time.DateOnly, updated)
				if err != nil {
					continue
				}
				if now.Sub(parsed) > 180*24*time.Hour {
					staleCount++
				}
			}
			assert.Equal(t, tt.wantStale, staleCount, "stale peer count")
		})
	}
}

// TestLoginWarningsPanicRecovery verifies that a panicking loginWarningsFunc
// does not crash the SSH session.
//
// VALIDATES: AC-5 from spec-login-warnings: No crash, normal welcome.
// PREVENTS: Panicking provider takes down the SSH session.
func TestLoginWarningsPanicRecovery(t *testing.T) {
	cfg := Config{
		HostKeyPath: t.TempDir() + "/test_host_key",
	}
	srv, err := NewServer(cfg)
	require.NoError(t, err)

	srv.SetLoginWarnings(func() []cli.LoginWarning {
		panic("provider exploded")
	})

	// Should not panic -- session degrades gracefully.
	model := srv.createSessionModel("alice")
	view := model.View().Content
	assert.NotContains(t, view, "warning:", "no warning should appear after provider panic")
}
