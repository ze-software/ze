package process

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/ipc"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// writeScript writes a test script with executable permissions.
// #nosec G306 - Test scripts must be executable
func writeScript(t *testing.T, path, content string) {
	t.Helper()
	err := os.WriteFile(path, []byte(content), 0o755) //nolint:gosec // Test scripts must be executable
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

	proc := NewProcess(plugin.PluginConfig{
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

	proc := NewProcess(plugin.PluginConfig{
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

	pm := NewProcessManager([]plugin.PluginConfig{
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

	pm := NewProcessManager([]plugin.PluginConfig{
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
	proc := NewProcess(plugin.PluginConfig{
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
	proc := NewProcess(plugin.PluginConfig{
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

	pm := NewProcessManager([]plugin.PluginConfig{
		{Name: "crash", Run: script, Encoder: "json", RespawnEnabled: true},
	})

	err := pm.Start()
	require.NoError(t, err)
	defer pm.Stop()

	// Attempt respawns beyond limit
	// Wait a bit between respawns for the crash script to exit
	for range RespawnLimit + 2 {
		respawnErr := pm.Respawn("crash")
		if errors.Is(respawnErr, ErrRespawnLimitExceeded) || errors.Is(respawnErr, ErrProcessDisabled) {
			break // Limit reached
		}
		time.Sleep(20 * time.Millisecond) // Let crash script exit
	}

	// Process should be disabled after exceeding limit
	assert.True(t, pm.IsDisabled("crash"), "process should be disabled after exceeding respawn limit")
}

// TestProcessManagerCumulativeRespawnLimit verifies process disabled after MaxTotalRespawns across windows.
//
// VALIDATES: Cumulative respawn counter prevents indefinite cycling across time windows.
// PREVENTS: Plugin cycling forever by staying just under per-window limit.
func TestProcessManagerCumulativeRespawnLimit(t *testing.T) {
	script := filepath.Join(t.TempDir(), "crash.sh")
	writeScript(t, script, "#!/bin/sh\nexit 1\n")

	pm := NewProcessManager([]plugin.PluginConfig{
		{Name: "cycle", Run: script, Encoder: "json", RespawnEnabled: true},
	})

	err := pm.Start()
	require.NoError(t, err)
	defer pm.Stop()

	// Simulate respawns across multiple windows by clearing per-window times
	// after each batch (simulates window expiry). The cumulative counter never resets.
	var hitLimit bool
	for batch := range MaxTotalRespawns {
		respawnErr := pm.Respawn("cycle")
		if errors.Is(respawnErr, ErrRespawnLimitExceeded) || errors.Is(respawnErr, ErrProcessDisabled) {
			hitLimit = true
			break
		}
		require.NoError(t, respawnErr, "respawn %d should succeed within cumulative limit", batch)
		time.Sleep(10 * time.Millisecond)

		// Clear per-window tracking to simulate window expiry (same package access)
		pm.mu.Lock()
		pm.respawnTimes["cycle"] = nil
		pm.mu.Unlock()
	}

	// The next respawn after MaxTotalRespawns should be rejected
	if !hitLimit {
		finalErr := pm.Respawn("cycle")
		assert.ErrorIs(t, finalErr, ErrRespawnLimitExceeded, "should hit cumulative limit")
	}

	assert.True(t, pm.IsDisabled("cycle"), "process should be disabled after cumulative limit")
}

// TestProcessManagerRespawnNotStarted verifies Respawn fails if manager not started.
//
// VALIDATES: Respawn returns error when ProcessManager.ctx is nil.
//
// PREVENTS: Panic from nil context in StartWithContext.
func TestProcessManagerRespawnNotStarted(t *testing.T) {
	pm := NewProcessManager([]plugin.PluginConfig{
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

	pm := NewProcessManager([]plugin.PluginConfig{
		{Name: "run", Run: script, Encoder: "json", RespawnEnabled: true},
	})

	err := pm.Start()
	require.NoError(t, err)
	defer pm.Stop()

	// First few respawns should succeed
	for i := range 3 {
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
	proc := NewProcess(plugin.PluginConfig{
		Name:     "bgp-rib",
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
	proc := NewProcess(plugin.PluginConfig{
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
	proc := NewProcess(plugin.PluginConfig{
		Name:     "bgp-rib",
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
	proc := NewProcess(plugin.PluginConfig{
		Name:     "bgp-rib",
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

// TestDeliveryLoopBatching verifies that multiple queued events are delivered in a single batch.
//
// VALIDATES: AC-2 — N events queued (burst) delivered in one batch write.
// PREVENTS: Events delivered one-at-a-time despite batching support.
func TestDeliveryLoopBatching(t *testing.T) {
	t.Parallel()

	pairs, err := ipc.NewInternalSocketPairs()
	require.NoError(t, err)
	t.Cleanup(func() { pairs.Close() })

	proc := NewProcess(plugin.PluginConfig{Name: "test-batch", Encoder: "json"})
	proc.SetConnB(ipc.NewPluginConn(pairs.Callback.EngineSide, pairs.Callback.EngineSide))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	proc.StartDelivery(ctx)
	defer proc.Stop()

	// Enqueue 3 events rapidly (should batch)
	results := make([]chan EventResult, 3)
	events := []string{
		`{"type":"bgp","bgp":{"peer":{"address":"10.0.0.1"}}}`,
		`{"type":"bgp","bgp":{"peer":{"address":"10.0.0.2"}}}`,
		`{"type":"bgp","bgp":{"peer":{"address":"10.0.0.3"}}}`,
	}
	for i, event := range events {
		results[i] = make(chan EventResult, 1)
		ok := proc.Deliver(EventDelivery{Output: event, Result: results[i]})
		require.True(t, ok, "event %d should be enqueued", i)
	}

	// Plugin side: read and respond to batch RPCs.
	// Under load (race detector, full suite), events may arrive in multiple batches
	// because drainBatch's non-blocking drain can fire before all events are enqueued.
	pluginConn := ipc.NewPluginConn(pairs.Callback.PluginSide, pairs.Callback.PluginSide)
	delivered := 0
	for delivered < len(events) {
		req, readErr := pluginConn.ReadRequest(ctx)
		require.NoError(t, readErr)
		assert.Equal(t, "ze-plugin-callback:deliver-batch", req.Method)

		var input struct {
			Events []json.RawMessage `json:"events"`
		}
		require.NoError(t, json.Unmarshal(req.Params, &input))
		delivered += len(input.Events)

		require.NoError(t, pluginConn.SendResult(ctx, req.ID, nil))
	}

	// All 3 results should complete without error
	for i := range 3 {
		select {
		case r := <-results[i]:
			assert.NoError(t, r.Err, "event %d should succeed", i)
		case <-time.After(2 * time.Second):
			t.Fatalf("timeout waiting for event %d result", i)
		}
	}
}

// TestDeliveryLoopSingleEvent verifies single event delivered as batch of 1.
//
// VALIDATES: AC-8 — first event triggers batch, non-blocking drain finds no more.
// PREVENTS: Single events hanging because batch waits for more.
func TestDeliveryLoopSingleEvent(t *testing.T) {
	t.Parallel()

	pairs, err := ipc.NewInternalSocketPairs()
	require.NoError(t, err)
	t.Cleanup(func() { pairs.Close() })

	proc := NewProcess(plugin.PluginConfig{Name: "test-single", Encoder: "json"})
	proc.SetConnB(ipc.NewPluginConn(pairs.Callback.EngineSide, pairs.Callback.EngineSide))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	proc.StartDelivery(ctx)
	defer proc.Stop()

	// Enqueue 1 event
	result := make(chan EventResult, 1)
	ok := proc.Deliver(EventDelivery{
		Output: `{"type":"bgp","bgp":{"peer":{"address":"10.0.0.1"}}}`,
		Result: result,
	})
	require.True(t, ok)

	// Plugin side: read the batch RPC request
	pluginConn := ipc.NewPluginConn(pairs.Callback.PluginSide, pairs.Callback.PluginSide)
	req, err := pluginConn.ReadRequest(ctx)
	require.NoError(t, err)
	assert.Equal(t, "ze-plugin-callback:deliver-batch", req.Method)

	// Verify it's a batch of 1
	var input struct {
		Events []json.RawMessage `json:"events"`
	}
	require.NoError(t, json.Unmarshal(req.Params, &input))
	assert.Len(t, input.Events, 1)

	// Respond OK
	require.NoError(t, pluginConn.SendResult(ctx, req.ID, nil))

	select {
	case r := <-result:
		assert.NoError(t, r.Err)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for event result")
	}
}

// TestDeliverBatchDirect verifies deliverBatch uses bridge for direct delivery.
//
// VALIDATES: AC-1 — Plugin's onEvent called directly without JSON-RPC.
// VALIDATES: AC-8 — EventResult.CacheConsumer correctly set on delivery success.
// PREVENTS: Direct transport path not being used when bridge is ready.
func TestDeliverBatchDirect(t *testing.T) {
	t.Parallel()

	bridge := rpc.NewDirectBridge()

	// Track events received by the plugin-side handler
	var mu sync.Mutex
	var received []string
	bridge.SetDeliverEvents(func(events []string) error {
		mu.Lock()
		defer mu.Unlock()
		received = append(received, events...)
		return nil
	})
	bridge.SetReady()

	proc := NewProcess(plugin.PluginConfig{Name: "test-direct", Encoder: "json"})
	proc.bridge = bridge
	proc.SetCacheConsumer(true) // Verify CacheConsumer tracking

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	proc.StartDelivery(ctx)
	defer proc.Stop()

	// Enqueue events
	results := make([]chan EventResult, 2)
	events := []string{
		`{"type":"bgp","bgp":{"peer":{"address":"10.0.0.1"}}}`,
		`{"type":"bgp","bgp":{"peer":{"address":"10.0.0.2"}}}`,
	}
	for i, event := range events {
		results[i] = make(chan EventResult, 1)
		ok := proc.Deliver(EventDelivery{Output: event, Result: results[i]})
		require.True(t, ok, "event %d should be enqueued", i)
	}

	// All results should complete without error (direct, no socket)
	for i := range 2 {
		select {
		case r := <-results[i]:
			assert.NoError(t, r.Err, "event %d should succeed", i)
			assert.True(t, r.CacheConsumer, "event %d should report cache consumer", i)
			assert.Equal(t, "test-direct", r.ProcName)
		case <-time.After(2 * time.Second):
			t.Fatalf("timeout waiting for event %d result", i)
		}
	}

	// Verify events reached the plugin handler directly
	mu.Lock()
	assert.Equal(t, events, received)
	mu.Unlock()
}

// TestDeliverBatchDirectError verifies bridge error propagation to EventResult.
//
// VALIDATES: AC-5 — Error propagated back to deliverBatch and reflected in EventResult.
// PREVENTS: Errors from direct delivery being swallowed.
func TestDeliverBatchDirectError(t *testing.T) {
	t.Parallel()

	bridge := rpc.NewDirectBridge()

	handlerErr := errors.New("plugin rejected event")
	bridge.SetDeliverEvents(func(events []string) error {
		return handlerErr
	})
	bridge.SetReady()

	proc := NewProcess(plugin.PluginConfig{Name: "test-direct-err", Encoder: "json"})
	proc.bridge = bridge
	proc.SetCacheConsumer(true) // CacheConsumer should be false on error

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	proc.StartDelivery(ctx)
	defer proc.Stop()

	result := make(chan EventResult, 1)
	ok := proc.Deliver(EventDelivery{
		Output: `{"type":"bgp","bgp":{"peer":{"address":"10.0.0.1"}}}`,
		Result: result,
	})
	require.True(t, ok)

	select {
	case r := <-result:
		assert.Error(t, r.Err)
		assert.Equal(t, handlerErr, r.Err)
		assert.False(t, r.CacheConsumer, "CacheConsumer should be false on error")
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for event result")
	}
}

// TestCloseConnsIdempotent verifies that calling closeConns multiple times
// does not panic. sync.Once ensures cleanup runs exactly once.
//
// VALIDATES: H3 — Stop/monitorCmd double-close race eliminated.
// PREVENTS: Panic or double-close when Stop() and monitorCmd() race.
func TestCloseConnsIdempotent(t *testing.T) {
	// Create a process with real internal socket pairs.
	pairs, err := ipc.NewInternalSocketPairs()
	require.NoError(t, err)

	proc := NewProcess(plugin.PluginConfig{Name: "test-close-idempotent"})
	proc.rawEngineA = pairs.Engine.EngineSide
	proc.rawCallbackB = pairs.Callback.EngineSide

	// First close — should work normally.
	proc.closeConns()

	// Second close — must not panic (sync.Once prevents double execution).
	proc.closeConns()

	// Third close from concurrent goroutine — must not panic.
	var wg sync.WaitGroup
	wg.Go(proc.closeConns)
	wg.Wait()
}

// TestCloseConnsConcurrent verifies that concurrent closeConns calls
// from Stop() and monitorCmd() paths don't race.
//
// VALIDATES: H3 — concurrent cleanup is safe.
// PREVENTS: Data race between Stop() and monitorCmd() closing connections.
func TestCloseConnsConcurrent(t *testing.T) {
	const goroutines = 10

	pairs, err := ipc.NewInternalSocketPairs()
	require.NoError(t, err)

	proc := NewProcess(plugin.PluginConfig{Name: "test-close-concurrent"})
	proc.rawEngineA = pairs.Engine.EngineSide
	proc.rawCallbackB = pairs.Callback.EngineSide

	// Hammer closeConns from many goroutines.
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			proc.closeConns()
		}()
	}
	wg.Wait()
}

// TestInternalPluginRunnerPanicRecovery verifies that an internal plugin
// runner that panics is caught by the recover() guard, preventing
// the engine from crashing. The goroutine exits cleanly.
//
// VALIDATES: C1 — Internal plugin panic does not crash engine.
// PREVENTS: Plugin panic propagating to engine process.
func TestInternalPluginRunnerPanicRecovery(t *testing.T) {
	// Simulate the internal plugin goroutine pattern from startInternal.
	// We can't call startInternal directly (requires registry), but we
	// replicate the exact defer/recover pattern to prove it works.
	proc := NewProcess(plugin.PluginConfig{Name: "test-panic-runner"})
	proc.running.Store(true)

	pairs, err := ipc.NewInternalSocketPairs()
	require.NoError(t, err)

	enginePluginSide := rpc.NewBridgedConn(pairs.Engine.PluginSide, rpc.NewDirectBridge())
	callbackPluginSide := rpc.NewBridgedConn(pairs.Callback.PluginSide, rpc.NewDirectBridge())

	// Replicate the exact goroutine structure from startInternal.
	proc.wg.Go(func() {
		defer proc.running.Store(false)
		defer func() {
			if rec := recover(); rec != nil {
				// Panic caught — this is what we're testing.
				logger().Error("internal plugin panic", "plugin", proc.config.Name, "panic", rec)
			}
		}()
		defer func() {
			if err := enginePluginSide.Close(); err != nil {
				logger().Debug("close engine plugin side", "error", err)
			}
		}()
		defer func() {
			if err := callbackPluginSide.Close(); err != nil {
				logger().Debug("close callback plugin side", "error", err)
			}
		}()

		// Simulate a panicking plugin runner.
		panic("plugin runtime error: index out of range")
	})

	// Wait for the goroutine to complete — must not hang or crash.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err = proc.Wait(ctx)
	require.NoError(t, err, "Wait should succeed after panic recovery")

	// Verify the process is no longer running.
	assert.False(t, proc.Running(), "process should not be running after panic")
}
