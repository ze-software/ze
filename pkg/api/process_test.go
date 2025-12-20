package api

import (
	"context"
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

	proc := NewProcess(ProcessConfig{
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

	proc := NewProcess(ProcessConfig{
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

	proc := NewProcess(ProcessConfig{
		Name:    "cmd",
		Run:     script,
		Encoder: "json",
	})

	err := proc.Start()
	require.NoError(t, err)
	defer proc.Stop()

	// Read command with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
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

	proc := NewProcess(ProcessConfig{
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

	pm := NewProcessManager([]ProcessConfig{
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

	pm := NewProcessManager([]ProcessConfig{
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
	proc := NewProcess(ProcessConfig{
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
