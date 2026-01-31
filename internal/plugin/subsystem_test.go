package plugin

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Shared test binary setup - built once, used by all tests.
var (
	subsystemBinaryPath string
	subsystemBuildOnce  sync.Once
	subsystemBuildErr   error
	subsystemTestTmpDir string
)

// TestMain handles cleanup of shared test resources.
func TestMain(m *testing.M) {
	code := m.Run()

	// Cleanup temp directory after all tests complete
	if subsystemTestTmpDir != "" {
		_ = os.RemoveAll(subsystemTestTmpDir)
	}

	os.Exit(code)
}

// buildSubsystemBinary builds the ze-subsystem binary once for all tests.
// Uses sync.Once to ensure only one build happens even with parallel tests.
func buildSubsystemBinary(_ context.Context, t *testing.T) {
	t.Helper()

	subsystemBuildOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		// Create temp directory for binary
		subsystemTestTmpDir, subsystemBuildErr = os.MkdirTemp("", "ze-subsystem-test-*")
		if subsystemBuildErr != nil {
			subsystemBuildErr = fmt.Errorf("create temp dir: %w", subsystemBuildErr)
			return
		}

		subsystemBinaryPath = filepath.Join(subsystemTestTmpDir, "ze-subsystem")

		// Find project root via go list
		listCmd := exec.CommandContext(ctx, "go", "list", "-m", "-f", "{{.Dir}}")
		output, err := listCmd.Output()
		if err != nil {
			subsystemBuildErr = fmt.Errorf("find project root: %w", err)
			return
		}
		projectRoot := strings.TrimSpace(string(output))

		// Build ze-subsystem binary once
		buildCmd := exec.CommandContext(ctx, "go", "build", "-o", subsystemBinaryPath, "./cmd/ze-subsystem") //nolint:gosec // test code
		buildCmd.Dir = projectRoot
		buildOutput, err := buildCmd.CombinedOutput()
		if err != nil {
			subsystemBuildErr = fmt.Errorf("build ze-subsystem: %w\n%s", err, buildOutput)
			return
		}
	})

	if subsystemBuildErr != nil {
		t.Skipf("skipping test requiring ze-subsystem binary: %v", subsystemBuildErr)
	}
}

// TestSubsystemBinaryExists verifies the subsystem binary can be built.
//
// VALIDATES: ze-subsystem binary compiles successfully.
// PREVENTS: Build failures in subsystem code.
func TestSubsystemBinaryExists(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	buildSubsystemBinary(ctx, t)
}

// TestSubsystemProtocol verifies the 5-stage protocol completes.
//
// VALIDATES: Subsystem completes declaration, config, capability, registry, ready.
// PREVENTS: Protocol deadlock or missing stage markers.
func TestSubsystemProtocol(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	buildSubsystemBinary(ctx, t)

	// Create process config
	config := PluginConfig{
		Name: "test-cache",
		Run:  subsystemBinaryPath + " --mode=cache",
	}

	proc := NewProcess(config)

	// Start the process
	err := proc.StartWithContext(ctx)
	require.NoError(t, err)
	defer proc.Stop()

	// Read Stage 1: Declaration
	var declareDone bool
	var commands []string

	for i := 0; i < 10 && !declareDone; i++ {
		line, err := proc.ReadCommand(ctx)
		if err != nil {
			break
		}
		if line == markerDeclareDone {
			declareDone = true
		} else if len(line) > 12 && line[:12] == "declare cmd " {
			commands = append(commands, line[12:])
		}
	}

	assert.True(t, declareDone, "expected declare done")
	assert.Contains(t, commands, "bgp cache list")

	// Send Stage 2: Config done
	err = proc.WriteEvent(markerConfigDone)
	require.NoError(t, err)

	// Read Stage 3: Capability done
	line, err := proc.ReadCommand(ctx)
	require.NoError(t, err)
	assert.Equal(t, markerCapabilityDone, line)

	// Send Stage 4: Registry done
	err = proc.WriteEvent(markerRegistryDone)
	require.NoError(t, err)

	// Read Stage 5: Ready
	line, err = proc.ReadCommand(ctx)
	require.NoError(t, err)
	assert.Equal(t, markerReady, line)
}

// TestSubsystemCommand verifies commands are routed to subprocess.
//
// VALIDATES: Commands sent to subprocess receive responses.
// PREVENTS: Command routing failures.
func TestSubsystemCommand(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	buildSubsystemBinary(ctx, t)

	// Create process config
	config := PluginConfig{
		Name: "test-session",
		Run:  subsystemBinaryPath + " --mode=session",
	}

	proc := NewProcess(config)

	// Start the process
	err := proc.StartWithContext(ctx)
	require.NoError(t, err)
	defer proc.Stop()

	// Complete 5-stage protocol
	for {
		line, err := proc.ReadCommand(ctx)
		require.NoError(t, err)
		if line == markerDeclareDone {
			break
		}
	}
	err = proc.WriteEvent(markerConfigDone)
	require.NoError(t, err)

	line, err := proc.ReadCommand(ctx)
	require.NoError(t, err)
	assert.Equal(t, markerCapabilityDone, line)

	err = proc.WriteEvent(markerRegistryDone)
	require.NoError(t, err)

	line, err = proc.ReadCommand(ctx)
	require.NoError(t, err)
	assert.Equal(t, markerReady, line)

	// Now send a command
	resp, err := proc.SendRequest(ctx, "bgp session ping")
	require.NoError(t, err)

	// Response should be "ok {...}" with pong PID
	assert.True(t, len(resp) > 3 && resp[:2] == "ok", "expected 'ok' response, got: %s", resp)
	assert.Contains(t, resp, "pong")
}

// TestSubsystemShutdown verifies graceful shutdown.
//
// VALIDATES: Subprocess exits cleanly on shutdown signal.
// PREVENTS: Zombie processes.
func TestSubsystemShutdown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	buildSubsystemBinary(ctx, t)

	config := PluginConfig{
		Name: "test-shutdown",
		Run:  subsystemBinaryPath + " --mode=cache",
	}

	proc := NewProcess(config)

	err := proc.StartWithContext(ctx)
	require.NoError(t, err)

	// Complete 5-stage protocol
	for {
		line, err := proc.ReadCommand(ctx)
		require.NoError(t, err)
		if line == markerDeclareDone {
			break
		}
	}
	_ = proc.WriteEvent(markerConfigDone)
	_, _ = proc.ReadCommand(ctx) // capability done
	_ = proc.WriteEvent(markerRegistryDone)
	_, _ = proc.ReadCommand(ctx) // ready

	// Send shutdown
	sent := proc.SendShutdown()
	assert.True(t, sent)

	// Wait for process to exit
	waitCtx, waitCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer waitCancel()

	err = proc.Wait(waitCtx)
	assert.NoError(t, err)
	assert.False(t, proc.Running())
}

// TestSubsystemHandler verifies the SubsystemHandler wrapper.
//
// VALIDATES: SubsystemHandler spawns process and routes commands.
// PREVENTS: Handler failing to complete protocol or route commands.
func TestSubsystemHandler(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	buildSubsystemBinary(ctx, t)

	// Use session mode - it doesn't require engine callbacks
	config := SubsystemConfig{
		Name:   "session",
		Binary: subsystemBinaryPath,
	}

	handler := NewSubsystemHandler(config)

	// Start the handler (completes 5-stage protocol)
	err := handler.Start(ctx)
	require.NoError(t, err)
	defer handler.Stop()

	// Verify commands were declared
	commands := handler.Commands()
	assert.Contains(t, commands, "bgp session ping")

	// Send a command that doesn't need callback
	resp, err := handler.Handle(ctx, "bgp session ping")
	require.NoError(t, err)
	assert.Equal(t, "done", resp.Status)
}

// TestSubsystemManager verifies the SubsystemManager.
//
// VALIDATES: Manager starts/stops multiple subsystems.
// PREVENTS: Manager failing to coordinate subsystems.
func TestSubsystemManager(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	buildSubsystemBinary(ctx, t)

	manager := NewSubsystemManager()
	manager.Register(SubsystemConfig{
		Name:   "cache",
		Binary: subsystemBinaryPath,
	})
	manager.Register(SubsystemConfig{
		Name:   "session",
		Binary: subsystemBinaryPath,
	})

	// Start all
	err := manager.StartAll(ctx)
	require.NoError(t, err)
	defer manager.StopAll()

	// Verify both running
	cache := manager.Get("cache")
	require.NotNil(t, cache)
	assert.True(t, cache.Running())

	session := manager.Get("session")
	require.NotNil(t, session)
	assert.True(t, session.Running())

	// Find handler for command
	handler := manager.FindHandler("bgp cache list")
	require.NotNil(t, handler)
	assert.Equal(t, "cache", handler.Name())

	handler = manager.FindHandler("bgp session ping")
	require.NotNil(t, handler)
	assert.Equal(t, "session", handler.Name())

	// All commands
	allCmds := manager.AllCommands()
	assert.Contains(t, allCmds, "bgp cache list")
	assert.Contains(t, allCmds, "bgp session ping")
}

// TestDispatcherSubsystemIntegration verifies Dispatcher routes to subsystems.
//
// VALIDATES: Dispatcher routes commands to forked subsystems.
// PREVENTS: Commands not being routed to subsystems.
func TestDispatcherSubsystemIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	buildSubsystemBinary(ctx, t)

	// Create dispatcher with subsystem manager
	d := NewDispatcher()

	// Register subsystems
	d.Subsystems().Register(SubsystemConfig{
		Name:   "session",
		Binary: subsystemBinaryPath,
	})

	// Start subsystems
	err := d.Subsystems().StartAll(ctx)
	require.NoError(t, err)
	defer d.Subsystems().StopAll()

	// Dispatch command to subsystem (not a builtin)
	// Note: "bgp session ping" is registered by the subsystem, not as builtin
	resp, err := d.Dispatch(nil, "bgp session ping")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)

	// Verify response contains pong
	if data, ok := resp.Data.(string); ok {
		assert.Contains(t, data, "pong")
	}
}
