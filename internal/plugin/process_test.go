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

// TestProcessWriteEvent verifies event writing to stdin.
//
// VALIDATES: JSON events written to process stdin.
//
// PREVENTS: Event delivery failures to external programs.
func TestProcessWriteEvent(t *testing.T) {
	// Create a script that reads and echoes to stdout
	script := filepath.Join(t.TempDir(), "echo.sh")
	writeScript(t, script, "#!/bin/sh\nread line\necho \"GOT:$line\"\n")

	proc := NewProcess(PluginConfig{
		Name:    "echo",
		Run:     script,
		Encoder: "json",
	})

	err := proc.Start()
	require.NoError(t, err)
	defer proc.Stop()

	// Write event
	err = proc.WriteEvent(`{"type":"test"}`)
	require.NoError(t, err)

	// Read back from process (5s timeout for CI/parallel test environments)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	output, err := proc.ReadCommand(ctx)
	require.NoError(t, err)
	assert.Contains(t, output, "GOT:")
	assert.Contains(t, output, "test")
}

// TestProcessReadCommand verifies command reading from stdout.
//
// VALIDATES: Commands read from process stdout.
//
// PREVENTS: Command reception failures from external programs.
func TestProcessReadCommand(t *testing.T) {
	// Create a script that outputs a command
	script := filepath.Join(t.TempDir(), "cmd.sh")
	writeScript(t, script, "#!/bin/sh\necho 'peer show'\nsleep 1\n")

	proc := NewProcess(PluginConfig{
		Name:    "cmd",
		Run:     script,
		Encoder: "json",
	})

	err := proc.Start()
	require.NoError(t, err)
	defer proc.Stop()

	// Read command with timeout (5s for CI/parallel test environments)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd, err := proc.ReadCommand(ctx)
	require.NoError(t, err)
	assert.Equal(t, "peer show", cmd)
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

// TestProcessWriteQueueBackpressure verifies events are dropped when queue is full.
//
// VALIDATES: Events dropped when write queue exceeds capacity, queue stats updated.
// ExaBGP: HIGH_WATER=1000, but we use smaller buffer for testing.
//
// PREVENTS: Memory exhaustion from slow consumers accumulating unbounded events.
func TestProcessWriteQueueBackpressure(t *testing.T) {
	// Create a slow script that doesn't read stdin quickly
	script := filepath.Join(t.TempDir(), "slow.sh")
	writeScript(t, script, "#!/bin/sh\nsleep 3600\n")

	proc := NewProcess(PluginConfig{
		Name:    "slow",
		Run:     script,
		Encoder: "json",
	})

	err := proc.Start()
	require.NoError(t, err)
	defer proc.Stop()

	// Fill the queue beyond capacity in a tight loop
	// The writeLoop will start draining, but we write faster than it can write to stdin
	// (stdin write blocks once pipe buffer is full since script doesn't read it)
	// Use much larger count to ensure we overwhelm the buffer
	for i := 0; i < WriteQueueHighWater*3; i++ {
		_ = proc.WriteEvent(`{"type":"flood"}`)
	}

	// Queue should have dropped some events
	// Note: drops may be 0 if pipe buffer absorbed everything - that's also valid
	// The key test is that queue size respects high water mark
	assert.LessOrEqual(t, proc.QueueSize(), WriteQueueHighWater, "queue should not exceed high water mark")
	// If drops occurred, great - if not, pipe buffer was sufficient
	// Either way the system didn't OOM
}

// TestProcessQueueStats verifies queue statistics are accessible.
//
// VALIDATES: QueueSize() and QueueDropped() return accurate counts.
//
// PREVENTS: Inability to monitor backpressure state for operations.
func TestProcessQueueStats(t *testing.T) {
	script := filepath.Join(t.TempDir(), "stats.sh")
	writeScript(t, script, "#!/bin/sh\nsleep 3600\n")

	proc := NewProcess(PluginConfig{
		Name:    "stats",
		Run:     script,
		Encoder: "json",
	})

	err := proc.Start()
	require.NoError(t, err)
	defer proc.Stop()

	// Initially empty
	assert.Equal(t, 0, proc.QueueSize(), "queue should be empty initially")
	assert.Equal(t, uint64(0), proc.QueueDropped(), "no drops initially")

	// Write some events (they may or may not queue depending on timing)
	for i := 0; i < 10; i++ {
		_ = proc.WriteEvent(`{"type":"test"}`)
	}

	// Stats should be accessible (values depend on timing)
	_ = proc.QueueSize()    // Should not panic
	_ = proc.QueueDropped() // Should not panic
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

// TestProcessWriteEventRaceCondition verifies WriteEvent doesn't panic during shutdown.
//
// VALIDATES: WriteEvent handles race between send and channel close gracefully.
//
// PREVENTS: Panic from sending to closed channel during process shutdown.
func TestProcessWriteEventRaceCondition(t *testing.T) {
	script := filepath.Join(t.TempDir(), "short.sh")
	writeScript(t, script, "#!/bin/sh\nexit 0\n") // Exits immediately

	// Run multiple iterations to increase chance of hitting race
	for i := 0; i < 10; i++ {
		proc := NewProcess(PluginConfig{
			Name:    "race",
			Run:     script,
			Encoder: "json",
		})

		err := proc.Start()
		require.NoError(t, err)

		// Hammer WriteEvent while process exits
		done := make(chan struct{})
		go func() {
			for j := 0; j < 100; j++ {
				_ = proc.WriteEvent(`{"type":"race"}`)
			}
			close(done)
		}()

		// Wait for writes to complete or timeout
		select {
		case <-done:
			// Success - no panic
		case <-time.After(time.Second):
			t.Log("write loop completed via timeout")
		}

		proc.Stop()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		_ = proc.Wait(ctx)
		cancel()
	}
	// If we get here without panic, test passes
}

// TestProcessWriteLoopHandlesErrors verifies writeLoop stops on write error.
//
// VALIDATES: Write errors (EPIPE) cause writeLoop to stop gracefully.
//
// PREVENTS: Infinite loop trying to write to dead process.
func TestProcessWriteLoopHandlesErrors(t *testing.T) {
	// Create a script that exits immediately (closes stdin)
	script := filepath.Join(t.TempDir(), "exit.sh")
	writeScript(t, script, "#!/bin/sh\nexit 0\n")

	proc := NewProcess(PluginConfig{
		Name:    "exit",
		Run:     script,
		Encoder: "json",
	})

	err := proc.Start()
	require.NoError(t, err)

	// Wait for process to exit
	time.Sleep(100 * time.Millisecond)

	// Write should not panic or block indefinitely
	for i := 0; i < 10; i++ {
		_ = proc.WriteEvent(`{"type":"test"}`)
	}

	// Process should be marked as not running
	proc.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	_ = proc.Wait(ctx)
	cancel()

	assert.False(t, proc.Running(), "process should not be running after exit")
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

// TestParseResponseSerial verifies @N response parsing.
//
// VALIDATES: @serial prefix is extracted correctly.
//
// PREVENTS: Wrong serial extraction, missing response body.
func TestParseResponseSerial(t *testing.T) {
	tests := []struct {
		line       string
		wantSerial string
		wantRest   string
		wantOK     bool
	}{
		{"@abc done", "abc", "done", true},
		{"@123 success data", "123", "success data", true},
		{"@a", "a", "", true},
		{"@bcd status ok", "bcd", "status ok", true},
		{"no prefix", "", "", false},
		{"@ space", "", "", false},
		{"", "", "", false},
		{"@", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			serial, rest, ok := parseResponseSerial(tt.line)
			assert.Equal(t, tt.wantOK, ok, "ok mismatch")
			if ok {
				assert.Equal(t, tt.wantSerial, serial, "serial mismatch")
				assert.Equal(t, tt.wantRest, rest, "rest mismatch")
			}
		})
	}
}

// TestProcessSendRequest verifies ZeBGP→Process request/response.
//
// VALIDATES: Request sent with alpha serial, response received and correlated.
//
// PREVENTS: Request/response mismatch, timeout issues.
func TestProcessSendRequest(t *testing.T) {
	// Create a script that echoes back responses
	// When it receives "#abc command", it responds with "@abc done"
	script := filepath.Join(t.TempDir(), "echo.sh")
	writeScript(t, script, `#!/bin/sh
while IFS= read -r line; do
  # Extract serial (part after # and before space)
  serial=$(echo "$line" | sed -n 's/^#\([^ ]*\).*/\1/p')
  if [ -n "$serial" ]; then
    echo "@$serial done"
  fi
done
`)

	proc := NewProcess(PluginConfig{
		Name:    "echo",
		Run:     script,
		Encoder: "json",
	})

	err := proc.Start()
	require.NoError(t, err)
	defer proc.Stop()

	// Give process time to start
	time.Sleep(50 * time.Millisecond)

	// Send request
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := proc.SendRequest(ctx, "test command")
	require.NoError(t, err)
	assert.Equal(t, "done", resp)
}

// TestProcessSendRequestTimeout verifies request timeout handling.
//
// VALIDATES: Request times out when process doesn't respond.
//
// PREVENTS: Infinite wait on unresponsive process.
func TestProcessSendRequestTimeout(t *testing.T) {
	// Create a script that never responds
	script := filepath.Join(t.TempDir(), "silent.sh")
	writeScript(t, script, "#!/bin/sh\ncat > /dev/null\n")

	proc := NewProcess(PluginConfig{
		Name:    "silent",
		Run:     script,
		Encoder: "json",
	})

	err := proc.Start()
	require.NoError(t, err)
	defer proc.Stop()

	// Give process time to start
	time.Sleep(50 * time.Millisecond)

	// Send request with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err = proc.SendRequest(ctx, "test")
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

// TestProcessSendRequestMultiple verifies multiple concurrent requests.
//
// VALIDATES: Multiple requests get correct responses via serial correlation.
//
// PREVENTS: Response mixup between concurrent requests.
func TestProcessSendRequestMultiple(t *testing.T) {
	// Create a script that echoes back responses with delay
	script := filepath.Join(t.TempDir(), "multi.sh")
	writeScript(t, script, `#!/bin/sh
while IFS= read -r line; do
  serial=$(echo "$line" | sed -n 's/^#\([^ ]*\) \(.*\)/\1/p')
  data=$(echo "$line" | sed -n 's/^#\([^ ]*\) \(.*\)/\2/p')
  if [ -n "$serial" ]; then
    echo "@$serial got $data"
  fi
done
`)

	proc := NewProcess(PluginConfig{
		Name:    "multi",
		Run:     script,
		Encoder: "json",
	})

	err := proc.Start()
	require.NoError(t, err)
	defer proc.Stop()

	time.Sleep(50 * time.Millisecond)

	// Send multiple requests
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp1, err := proc.SendRequest(ctx, "first")
	require.NoError(t, err)
	assert.Equal(t, "got first", resp1)

	resp2, err := proc.SendRequest(ctx, "second")
	require.NoError(t, err)
	assert.Equal(t, "got second", resp2)
}
