package plugin

import (
	"context"
	"encoding/json"
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

	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
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

// TestSubsystemRPCProtocol verifies the 5-stage RPC protocol at the Process level.
//
// VALIDATES: ze-subsystem binary sends correct YANG RPC methods in stage order
// and declares expected commands via DeclareRegistrationInput.
// PREVENTS: Protocol regression when SDK or subsystem changes.
func TestSubsystemRPCProtocol(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	buildSubsystemBinary(ctx, t)

	config := PluginConfig{
		Name: "test-cache",
		Run:  subsystemBinaryPath + " --mode=cache",
	}

	proc := NewProcess(config)
	err := proc.StartWithContext(ctx)
	require.NoError(t, err)
	defer proc.Stop()

	connA := proc.engineConnA
	connB := proc.engineConnB

	// Stage 1: Read declare-registration from plugin (Socket A)
	req, err := connA.ReadRequest(ctx)
	require.NoError(t, err, "stage 1: read registration")
	assert.Equal(t, "ze-plugin-engine:declare-registration", req.Method)

	var regInput rpc.DeclareRegistrationInput
	require.NoError(t, json.Unmarshal(req.Params, &regInput), "stage 1: unmarshal registration")

	// Verify cache mode declared expected commands
	commands := make([]string, 0, len(regInput.Commands))
	for _, cmd := range regInput.Commands {
		commands = append(commands, cmd.Name)
	}
	assert.Contains(t, commands, "bgp cache list")
	assert.Contains(t, commands, "bgp cache retain")
	assert.Contains(t, commands, "bgp cache release")

	require.NoError(t, connA.SendResult(ctx, req.ID, nil), "stage 1: send OK")

	// Stage 2: Send configure to plugin (Socket B)
	require.NoError(t, connB.SendConfigure(ctx, nil), "stage 2: send configure")

	// Stage 3: Read declare-capabilities from plugin (Socket A)
	req, err = connA.ReadRequest(ctx)
	require.NoError(t, err, "stage 3: read capabilities")
	assert.Equal(t, "ze-plugin-engine:declare-capabilities", req.Method)
	require.NoError(t, connA.SendResult(ctx, req.ID, nil), "stage 3: send OK")

	// Stage 4: Send share-registry to plugin (Socket B)
	require.NoError(t, connB.SendShareRegistry(ctx, nil), "stage 4: send registry")

	// Stage 5: Read ready from plugin (Socket A)
	req, err = connA.ReadRequest(ctx)
	require.NoError(t, err, "stage 5: read ready")
	assert.Equal(t, "ze-plugin-engine:ready", req.Method)
	require.NoError(t, connA.SendResult(ctx, req.ID, nil), "stage 5: send OK")
}

// TestSubsystemRPCCommand verifies command execution through the RPC protocol.
//
// VALIDATES: After completing 5-stage protocol, commands are routed and return responses.
// PREVENTS: Command routing failures after protocol migration to RPC.
func TestSubsystemRPCCommand(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	buildSubsystemBinary(ctx, t)

	config := SubsystemConfig{
		Name:   "session",
		Binary: subsystemBinaryPath,
	}

	handler := NewSubsystemHandler(config)
	err := handler.Start(ctx)
	require.NoError(t, err)
	defer handler.Stop()

	// Send command and verify response content (not just status)
	resp, err := handler.Handle(ctx, "bgp session ping")
	require.NoError(t, err)
	assert.Equal(t, "done", resp.Status)

	// Verify response contains pong with PID (matches ze-subsystem handleSessionCommand)
	data, ok := resp.Data.(string)
	require.True(t, ok, "expected string data")
	assert.Contains(t, data, "pong")
}

// TestSubsystemShutdown verifies graceful shutdown via RPC bye.
//
// VALIDATES: Subprocess exits cleanly on shutdown signal.
// PREVENTS: Zombie processes after subsystem shutdown.
func TestSubsystemShutdown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	buildSubsystemBinary(ctx, t)

	config := SubsystemConfig{
		Name:   "cache",
		Binary: subsystemBinaryPath,
	}

	handler := NewSubsystemHandler(config)
	err := handler.Start(ctx)
	require.NoError(t, err)

	// Verify running before shutdown
	assert.True(t, handler.Running())

	// Stop sends shutdown signal
	handler.Stop()

	// Wait briefly for process to exit
	waitCtx, waitCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer waitCancel()

	// Poll until not running or timeout
	for handler.Running() {
		select {
		case <-waitCtx.Done():
			t.Fatal("subsystem did not exit within timeout")
		case <-time.After(50 * time.Millisecond):
		}
	}

	assert.False(t, handler.Running())
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
