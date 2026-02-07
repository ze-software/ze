package plugin

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeScript writes a test script with executable permissions.
// #nosec G306 - Test scripts must be executable
func writeScript(t *testing.T, path, content string) {
	t.Helper()
	err := os.WriteFile(path, []byte(content), 0755) //nolint:gosec // Test scripts must be executable
	require.NoError(t, err)
}

// TestProcessStart verifies process spawning.
//
// VALIDATES: Process starts with correct command.
//
// PREVENTS: Process spawn failures blocking API functionality.
func TestProcessStart(t *testing.T) {
	// Create a simple script that exits immediately
	script := filepath.Join(t.TempDir(), "test.sh")
	writeScript(t, script, "#!/bin/sh\nexit 0\n")

	proc := NewProcess(PluginConfig{
		Name:    "test",
		Run:     script,
		Encoder: "json",
	})

	err := proc.Start()
	require.NoError(t, err)
	assert.True(t, proc.Running())

	// Wait for process to exit
	time.Sleep(50 * time.Millisecond)
	proc.Stop()
}

// TestProcessShutdown verifies clean process termination.
//
// VALIDATES: Process receives signal and terminates.
//
// PREVENTS: Orphaned processes after API shutdown.
func TestProcessShutdown(t *testing.T) {
	// Create a script that sleeps forever
	script := filepath.Join(t.TempDir(), "sleep.sh")
	writeScript(t, script, "#!/bin/sh\nsleep 3600\n")

	proc := NewProcess(PluginConfig{
		Name:    "sleep",
		Run:     script,
		Encoder: "json",
	})

	err := proc.Start()
	require.NoError(t, err)
	assert.True(t, proc.Running())

	// Stop should terminate the process
	proc.Stop()

	// Wait should complete quickly
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err = proc.Wait(ctx)
	require.NoError(t, err)
	assert.False(t, proc.Running())
}

// TestProcessManagerStartAll verifies all processes start.
//
// VALIDATES: ProcessManager starts all configured processes.
//
// PREVENTS: Some processes not starting.
func TestProcessManagerStartAll(t *testing.T) {
	// Create test scripts
	script1 := filepath.Join(t.TempDir(), "p1.sh")
	script2 := filepath.Join(t.TempDir(), "p2.sh")
	for _, s := range []string{script1, script2} {
		writeScript(t, s, "#!/bin/sh\nsleep 10\n")
	}

	pm := NewProcessManager([]PluginConfig{
		{Name: "p1", Run: script1, Encoder: "json"},
		{Name: "p2", Run: script2, Encoder: "json"},
	})

	err := pm.Start()
	require.NoError(t, err)
	defer pm.Stop()

	assert.Equal(t, 2, pm.ProcessCount())
	assert.True(t, pm.IsRunning("p1"))
	assert.True(t, pm.IsRunning("p2"))
}

// TestProcessManagerStopAll verifies all processes stop.
//
// VALIDATES: ProcessManager stops all processes on shutdown.
//
// PREVENTS: Orphaned processes.
func TestProcessManagerStopAll(t *testing.T) {
	script := filepath.Join(t.TempDir(), "sleep.sh")
	writeScript(t, script, "#!/bin/sh\nsleep 3600\n")

	pm := NewProcessManager([]PluginConfig{
		{Name: "p1", Run: script, Encoder: "json"},
		{Name: "p2", Run: script, Encoder: "json"},
	})

	err := pm.Start()
	require.NoError(t, err)

	pm.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err = pm.Wait(ctx)
	require.NoError(t, err)

	assert.Equal(t, 0, pm.ProcessCount())
}

// TestProcessNotFound verifies handling of missing executable.
//
// VALIDATES: Process exits quickly when command not found.
//
// PREVENTS: Silent failures from misconfigured processes.
func TestProcessNotFound(t *testing.T) {
	proc := NewProcess(PluginConfig{
		Name:    "missing",
		Run:     "/nonexistent/path/to/script",
		Encoder: "json",
	})

	// Start will succeed because /bin/sh -c starts
	// but the script will fail immediately
	err := proc.Start()
	if err != nil {
		// Some systems may fail at start
		assert.False(t, proc.Running())
		return
	}

	// Wait for process to exit due to missing script
	time.Sleep(100 * time.Millisecond)
	assert.False(t, proc.Running(), "process should exit when script not found")
}

// TestProcessManagerNoProcesses verifies empty config handling.
//
// VALIDATES: ProcessManager works with no configured processes.
//
// PREVENTS: Panics when no processes configured.
func TestProcessManagerNoProcesses(t *testing.T) {
	pm := NewProcessManager(nil)

	err := pm.Start()
	require.NoError(t, err)
	defer pm.Stop()

	assert.Equal(t, 0, pm.ProcessCount())
}

// TestProcessSyncState verifies sync state management on Process.
//
// VALIDATES: Process tracks sync enabled/disabled state correctly.
// Default is disabled (false).
//
// PREVENTS: Missing sync state, incorrect default, sync always on
// causing unnecessary waits.
func TestProcessSyncState(t *testing.T) {
	proc := NewProcess(PluginConfig{
		Name:    "test",
		Run:     "echo test",
		Encoder: "json",
	})

	// Default should be disabled
	assert.False(t, proc.SyncEnabled(), "sync should be disabled by default")

	// Enable sync
	proc.SetSync(true)
	assert.True(t, proc.SyncEnabled(), "sync should be enabled after SetSync(true)")

	// Disable sync
	proc.SetSync(false)
	assert.False(t, proc.SyncEnabled(), "sync should be disabled after SetSync(false)")
}

// TestProcessManagerRespawnLimit verifies process disabled after too many respawns.
//
// VALIDATES: Process disabled after 5 respawns within 60 seconds.
// ExaBGP: respawn_number=5, respawn_timemask covers ~63 seconds.
//
// PREVENTS: Infinite respawn loops consuming resources from crashing processes.
func TestProcessManagerRespawnLimit(t *testing.T) {
	// Create a script that exits immediately
	script := filepath.Join(t.TempDir(), "crash.sh")
	writeScript(t, script, "#!/bin/sh\nexit 1\n")

	pm := NewProcessManager([]PluginConfig{
		{Name: "crash", Run: script, Encoder: "json", RespawnEnabled: true},
	})

	err := pm.Start()
	require.NoError(t, err)
	defer pm.Stop()

	// Attempt respawns beyond limit
	// Wait a bit between respawns for the crash script to exit
	for i := 0; i < RespawnLimit+2; i++ {
		respawnErr := pm.Respawn("crash")
		if errors.Is(respawnErr, ErrRespawnLimitExceeded) || errors.Is(respawnErr, ErrProcessDisabled) {
			break // Limit reached
		}
		time.Sleep(20 * time.Millisecond) // Let crash script exit
	}

	// Process should be disabled after exceeding limit
	assert.True(t, pm.IsDisabled("crash"), "process should be disabled after exceeding respawn limit")
}

// TestProcessManagerRespawnNotStarted verifies Respawn fails if manager not started.
//
// VALIDATES: Respawn returns error when ProcessManager.ctx is nil.
//
// PREVENTS: Panic from nil context in StartWithContext.
func TestProcessManagerRespawnNotStarted(t *testing.T) {
	pm := NewProcessManager([]PluginConfig{
		{Name: "test", Run: "echo test", Encoder: "json", RespawnEnabled: true},
	})

	// Don't call pm.Start() - ctx is nil

	err := pm.Respawn("test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not started")
}

// TestProcessManagerRespawnSuccess verifies respawn works within limits.
//
// VALIDATES: Process can be respawned if within limit.
//
// PREVENTS: Valid respawn attempts being rejected.
func TestProcessManagerRespawnSuccess(t *testing.T) {
	// Create a script that runs until stopped
	script := filepath.Join(t.TempDir(), "run.sh")
	writeScript(t, script, "#!/bin/sh\nsleep 3600\n")

	pm := NewProcessManager([]PluginConfig{
		{Name: "run", Run: script, Encoder: "json", RespawnEnabled: true},
	})

	err := pm.Start()
	require.NoError(t, err)
	defer pm.Stop()

	// First few respawns should succeed
	for i := 0; i < 3; i++ {
		err := pm.Respawn("run")
		require.NoError(t, err, "respawn %d should succeed", i)
		time.Sleep(10 * time.Millisecond) // Let process start
	}

	assert.True(t, pm.IsRunning("run"), "process should be running after respawn")
	assert.False(t, pm.IsDisabled("run"), "process should not be disabled within limit")
}

// TestProcessInternalPlugin verifies internal plugins run in-process.
//
// VALIDATES: Internal plugins start via goroutine, not fork.
// PREVENTS: Internal plugins accidentally forking.
func TestProcessInternalPlugin(t *testing.T) {
	proc := NewProcess(PluginConfig{
		Name:     "rib",
		Internal: true,
		Encoder:  "json",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := proc.StartWithContext(ctx)
	require.NoError(t, err)
	assert.True(t, proc.Running())
	assert.Nil(t, proc.cmd, "internal plugin should not have exec.Cmd")

	proc.Stop()
}

// TestProcessInternalPluginUnknown verifies error for unknown internal plugin.
//
// VALIDATES: Unknown internal plugin returns error.
// PREVENTS: Silent failure for unknown internal plugins.
func TestProcessInternalPluginUnknown(t *testing.T) {
	proc := NewProcess(PluginConfig{
		Name:     "nonexistent",
		Internal: true,
	})

	err := proc.Start()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown internal plugin")
}

// TestProcessInternalPluginStop verifies internal plugins stop cleanly.
//
// VALIDATES: Stop() closes connections, causing plugin to exit.
// PREVENTS: Internal plugins hanging on Stop().
func TestProcessInternalPluginStop(t *testing.T) {
	proc := NewProcess(PluginConfig{
		Name:     "rib",
		Internal: true,
		Encoder:  "json",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := proc.StartWithContext(ctx)
	require.NoError(t, err)
	assert.True(t, proc.Running())

	// Stop should close connections and cause plugin to exit
	proc.Stop()

	// Wait for plugin to exit (with timeout)
	waitCtx, waitCancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer waitCancel()

	err = proc.Wait(waitCtx)
	require.NoError(t, err, "internal plugin should exit after Stop()")
	assert.False(t, proc.Running())
}

// TestProcessInternalPluginSocketPairs verifies internal plugins use socket pairs.
//
// VALIDATES: Internal plugin transport uses DualSocketPair instead of io.Pipe.
// PREVENTS: Regression to io.Pipe transport after socket pair migration.
func TestProcessInternalPluginSocketPairs(t *testing.T) {
	proc := NewProcess(PluginConfig{
		Name:     "rib",
		Internal: true,
		Encoder:  "json",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := proc.StartWithContext(ctx)
	require.NoError(t, err)
	assert.True(t, proc.Running())

	// Socket pairs should be allocated for internal plugins
	assert.NotNil(t, proc.sockets, "internal plugin should use socket pairs")

	proc.Stop()

	waitCtx, waitCancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer waitCancel()

	err = proc.Wait(waitCtx)
	require.NoError(t, err)
}
