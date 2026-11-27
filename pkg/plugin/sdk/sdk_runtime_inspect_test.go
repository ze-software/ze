// Design: docs/architecture/api/process-protocol.md -- SDK runtime-inspect independent worker tests.
// Related: sdk_runtime_inspect.go -- independent worker and built-in collectors.
package sdk

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/core/flightrec"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const (
	testTransportWire        = "wire"
	testTransportTyped       = "typed"
	testLogTag               = "inspect-test"
	testLogLevelWarn         = "warn"
	testMethodExecuteCommand = "ze-plugin-callback:execute-command"
)

func runStartedPlugin(t *testing.T, ctx context.Context, typed bool) (*Plugin, *engineSide, *ipc.PluginConn, chan error) {
	t.Helper()

	var (
		p      *Plugin
		engine *engineSide
		bridge *rpc.DirectBridge
	)
	if typed {
		p, engine, bridge = newBridgedTestPair(t)
	} else {
		p, engine = newTestPair(t)
	}
	conn := ipc.NewMuxPluginConn(engine.mux)
	if typed {
		conn.SetBridge(bridge)
	}

	started := make(chan struct{})
	p.OnStarted(func(context.Context) error {
		close(started)
		return nil
	})

	done := make(chan error, 1)
	go func() {
		done <- p.Run(ctx, &Registration{})
	}()

	req := readEngineRequest(t, ctx, engine.mux)
	require.Equal(t, rpc.MethodDeclareRegistration, req.Method)
	require.NoError(t, engine.mux.SendResult(ctx, req.ID, &rpc.DeclareRegistrationOutput{ProtocolVersion: rpc.ProtocolVersion}))
	completeStartupFromStage2(t, ctx, engine)

	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal("plugin never reached OnStarted")
	}
	return p, engine, conn, done
}

func assertAllSnapshotKinds(t *testing.T, ctx context.Context, conn *ipc.PluginConn, logComponent, logMarker string) {
	t.Helper()
	pid := os.Getpid()

	// 1. goroutines
	gorOut, err := conn.SendRuntimeInspect(ctx, rpc.RuntimeInspectInput{Kind: rpc.RuntimeInspectGoroutines})
	require.NoError(t, err)
	require.Equal(t, rpc.RuntimeInspectGoroutines, gorOut.Kind)
	require.Equal(t, pid, gorOut.PID)
	require.Contains(t, gorOut.GoroutineStacks, "goroutine ")

	// 2. runtime-stats
	statsOut, err := conn.SendRuntimeInspect(ctx, rpc.RuntimeInspectInput{Kind: rpc.RuntimeInspectRuntimeStats})
	require.NoError(t, err)
	require.Equal(t, rpc.RuntimeInspectRuntimeStats, statsOut.Kind)
	require.Equal(t, pid, statsOut.PID)
	require.NotNil(t, statsOut.RuntimeStats)
	require.Positive(t, statsOut.RuntimeStats.NumGoroutines)
	require.Positive(t, statsOut.RuntimeStats.NumCPU)
	require.Positive(t, statsOut.RuntimeStats.MaxProcs)
	require.Equal(t, runtime.Version(), statsOut.RuntimeStats.GoVersion)

	// 3. snapshot profiles
	prevMutex := runtime.SetMutexProfileFraction(-1)
	prevBlock := flightrec.BlockProfileRate()
	t.Cleanup(func() {
		runtime.SetMutexProfileFraction(prevMutex)
		flightrec.SetBlockProfileRate(prevBlock)
	})
	runtime.SetMutexProfileFraction(10)
	flightrec.SetBlockProfileRate(100_000)

	for _, profileType := range []string{
		rpc.RuntimeInspectProfileHeap,
		rpc.RuntimeInspectProfileGoroutine,
		rpc.RuntimeInspectProfileAllocs,
		rpc.RuntimeInspectProfileMutex,
		rpc.RuntimeInspectProfileBlock,
	} {
		profOut, profErr := conn.SendRuntimeInspect(ctx, rpc.RuntimeInspectInput{
			Kind:        rpc.RuntimeInspectProfile,
			ProfileType: profileType,
		})
		require.NoError(t, profErr, "profile %s", profileType)
		require.Equal(t, rpc.RuntimeInspectProfile, profOut.Kind)
		require.Equal(t, pid, profOut.PID)
		require.Equal(t, profileType, profOut.ProfileType)
		decoded, decErr := base64.StdEncoding.DecodeString(profOut.ProfileData)
		require.NoError(t, decErr)
		require.NotEmpty(t, decoded)
		switch profileType {
		case rpc.RuntimeInspectProfileMutex:
			require.NotNil(t, profOut.SamplingRate)
			require.Equal(t, 10, *profOut.SamplingRate)
		case rpc.RuntimeInspectProfileBlock:
			require.NotNil(t, profOut.SamplingRate)
			require.Equal(t, 100_000, *profOut.SamplingRate)
		default:
			require.Nil(t, profOut.SamplingRate)
		}
	}

	// 4. log-recent
	logOut, err := conn.SendRuntimeInspect(ctx, rpc.RuntimeInspectInput{
		Kind:         rpc.RuntimeInspectLogRecent,
		LogLevel:     testLogLevelWarn,
		LogComponent: logComponent,
		LogCount:     16,
	})
	require.NoError(t, err)
	require.Equal(t, rpc.RuntimeInspectLogRecent, logOut.Kind)
	require.Equal(t, pid, logOut.PID)
	found := false
	for _, entry := range logOut.LogEntries {
		if entry.Message != logMarker {
			continue
		}
		found = true
		require.Equal(t, testLogLevelWarn, entry.Level)
		require.Equal(t, logComponent, entry.Component)
		require.Equal(t, testLogTag, entry.Attributes["tag"])
		break
	}
	require.True(t, found, "expected log marker %q in log-recent entries", logMarker)
}

// TestRuntimeInspectAutoStartAllKinds verifies Plugin.Run starts runtime-inspect
// automatically with zero per-plugin boilerplate on both transports.
//
// VALIDATES: AC-1, sdk.Plugin.Run automatically serves goroutines, profile,
// log-recent, and runtime-stats over both DirectBridge and MuxConn.
// PREVENTS: requiring manual per-plugin registration for runtime inspection.
func TestRuntimeInspectAutoStartAllKinds(t *testing.T) {
	const logComponent = "diag3-autostart"
	const logMarker = "diag3 autostart log marker"
	slogutil.Logger(logComponent).Warn(logMarker, "tag", testLogTag)

	for _, typed := range []bool{false, true} {
		mode := testTransportWire
		if typed {
			mode = testTransportTyped
		}
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()

			_, _, conn, done := runStartedPlugin(t, ctx, typed)
			assertAllSnapshotKinds(t, ctx, conn, logComponent, logMarker)

			cancel()
			require.NoError(t, <-done)
		})
	}
}

// TestRuntimeInspectBypassesDeadlockedEventLoop proves runtime-inspect keeps
// answering all four kinds while the plugin's ordinary OnExecuteCommand handler
// is blocked indefinitely on both DirectBridge and MuxConn.
//
// VALIDATES: AC-2, independent runtime-inspect worker survives a deadlocked
// ordinary eventLoop on both DirectBridge and MuxConn.
// PREVENTS: diagnostic requests stalling behind blocked ordinary callbacks.
func TestRuntimeInspectBypassesDeadlockedEventLoop(t *testing.T) {
	const logComponent = "diag3-deadlock"
	const logMarker = "diag3 deadlock log marker"
	slogutil.Logger(logComponent).Warn(logMarker, "tag", testLogTag)

	for _, typed := range []bool{false, true} {
		mode := testTransportWire
		if typed {
			mode = testTransportTyped
		}
		t.Run(mode, func(t *testing.T) {
			p, engine := newTestPair(t)
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()

			bridge := rpc.NewDirectBridge()
			conn := ipc.NewMuxPluginConn(engine.mux)
			if typed {
				p.bridge = bridge
				conn.SetBridge(bridge)
			}

			stop, err := p.startRuntimeInspectWorker(ctx)
			require.NoError(t, err)
			defer func() { require.NoError(t, stop()) }()

			entered, release := make(chan struct{}), make(chan struct{})
			p.OnExecuteCommand(func(string, string, []string, string, []PipeOperation) (string, any, error) {
				close(entered)
				select {
				case <-release:
				case <-ctx.Done():
				}
				return rpc.StatusOK, nil, nil
			})

			loopDone, cmdDone := make(chan error, 1), make(chan error, 1)
			if typed {
				p.activateBridge()
				go func() { loopDone <- p.bridgeEventLoop(ctx) }()
				go func() {
					_, callErr := bridge.SendCallback(ctx, testMethodExecuteCommand, json.RawMessage(`{"command":"held"}`))
					cmdDone <- callErr
				}()
			} else {
				go func() { loopDone <- p.eventLoop(ctx) }()
				go func() {
					answer, callErr := engine.mux.CallAnswer(ctx, testMethodExecuteCommand, map[string]any{"command": "held"})
					if callErr == nil {
						for range answer.Records {
						}
						callErr = answer.Err()
					}
					cmdDone <- callErr
				}()
			}

			select {
			case <-entered:
			case <-ctx.Done():
				t.Fatal("ordinary execute-command callback never entered")
			}

			assertAllSnapshotKinds(t, ctx, conn, logComponent, logMarker)

			close(release)
			require.NoError(t, <-cmdDone)
			cancel()
			<-loopDone
		})
	}
}

// TestRuntimeInspectCPUProfileAndConcurrency covers real CPU profiling,
// TryLock collision without connection closure, and prompt cancellation.
//
// VALIDATES: AC-3, concurrent CPU profile returns "CPU profile already in
// progress" without closing MuxConn, and canceled 10s CPU profiles stop in
// under 1s on both transports.
// PREVENTS: overlapping CPU profiles or hung profiling blocking plugin shutdown.
func TestRuntimeInspectCPUProfileAndConcurrency(t *testing.T) {
	t.Run("capture-1s-wire", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()

		p, engine := newTestPair(t)
		conn := ipc.NewMuxPluginConn(engine.mux)
		stop, err := p.startRuntimeInspectWorker(ctx)
		require.NoError(t, err)
		defer func() { require.NoError(t, stop()) }()

		out, err := conn.SendRuntimeInspect(ctx, rpc.RuntimeInspectInput{
			Kind:        rpc.RuntimeInspectProfile,
			ProfileType: rpc.RuntimeInspectProfileCPU,
			DurationMS:  1000,
		})
		require.NoError(t, err)
		require.Equal(t, rpc.RuntimeInspectProfile, out.Kind)
		require.Equal(t, rpc.RuntimeInspectProfileCPU, out.ProfileType)
		require.Equal(t, os.Getpid(), out.PID)
		decoded, decErr := base64.StdEncoding.DecodeString(out.ProfileData)
		require.NoError(t, decErr)
		require.NotEmpty(t, decoded)
	})

	t.Run("collision-preserves-connection", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()

		p, engine := newTestPair(t)
		conn := ipc.NewMuxPluginConn(engine.mux)
		stop, err := p.startRuntimeInspectWorker(ctx)
		require.NoError(t, err)
		defer func() { require.NoError(t, stop()) }()

		sdkCPUProfileMu.Lock()
		_, err = conn.SendRuntimeInspect(ctx, rpc.RuntimeInspectInput{
			Kind:        rpc.RuntimeInspectProfile,
			ProfileType: rpc.RuntimeInspectProfileCPU,
			DurationMS:  1000,
		})
		sdkCPUProfileMu.Unlock()
		require.Error(t, err)
		require.Contains(t, err.Error(), errSDKCPUProfileInProgress.Error())

		// Connection must remain healthy after CPU profile collision.
		out, err := conn.SendRuntimeInspect(ctx, rpc.RuntimeInspectInput{Kind: rpc.RuntimeInspectRuntimeStats})
		require.NoError(t, err)
		require.Equal(t, os.Getpid(), out.PID)
	})

	for _, typed := range []bool{false, true} {
		mode := testTransportWire
		if typed {
			mode = testTransportTyped
		}
		t.Run("cancel-stops-promptly/"+mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()

			var (
				p      *Plugin
				engine *engineSide
				bridge *rpc.DirectBridge
			)
			if typed {
				p, engine, bridge = newBridgedTestPair(t)
			} else {
				p, engine = newTestPair(t)
			}
			conn := ipc.NewMuxPluginConn(engine.mux)
			if typed {
				conn.SetBridge(bridge)
			}

			stop, err := p.startRuntimeInspectWorker(ctx)
			require.NoError(t, err)
			defer func() { require.NoError(t, stop()) }()
			if typed {
				bridge.SetReady()
			}

			callCtx, callCancel := context.WithTimeout(ctx, 150*time.Millisecond)
			defer callCancel()

			start := time.Now()
			_, err = conn.SendRuntimeInspect(callCtx, rpc.RuntimeInspectInput{
				Kind:        rpc.RuntimeInspectProfile,
				ProfileType: rpc.RuntimeInspectProfileCPU,
				DurationMS:  10000,
			})
			elapsed := time.Since(start)
			require.Error(t, err)
			require.Less(t, elapsed, time.Second)

			// Wait briefly for the profiler lock to clear and confirm a follow-up
			// inspection succeeds on the exact same connection.
			out, err := conn.SendRuntimeInspect(ctx, rpc.RuntimeInspectInput{Kind: rpc.RuntimeInspectRuntimeStats})
			require.NoError(t, err)
			require.Equal(t, os.Getpid(), out.PID)
		})
	}
}

// TestRuntimeInspectFaultIsolation verifies that collector panics, expired
// deadlines, and queue saturation never tear down DirectBridge or MuxConn.
//
// VALIDATES: AC-3, diagnostic panics, expired deadlines, and queue bursts leave
// the plugin's production connection and ordinary callbacks alive.
// PREVENTS: diagnostic failures from causing production plugin disconnects.
func TestRuntimeInspectFaultIsolation(t *testing.T) {
	for _, typed := range []bool{false, true} {
		mode := testTransportWire
		if typed {
			mode = testTransportTyped
		}
		t.Run("panic-and-deadline/"+mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()

			var (
				p      *Plugin
				engine *engineSide
				bridge *rpc.DirectBridge
			)
			if typed {
				p, engine, bridge = newBridgedTestPair(t)
			} else {
				p, engine = newTestPair(t)
			}
			conn := ipc.NewMuxPluginConn(engine.mux)
			if typed {
				conn.SetBridge(bridge)
			}

			var shouldPanic atomic.Bool
			shouldPanic.Store(true)
			stop, err := p.startRuntimeInspectWorkerWithCollector(ctx, func(c context.Context, in rpc.RuntimeInspectInput) (*rpc.RuntimeInspectOutput, error) {
				if shouldPanic.Swap(false) {
					panic("synthetic diagnostic panic")
				}
				return collectRuntimeInspect(c, in)
			})
			require.NoError(t, err)
			defer func() { require.NoError(t, stop()) }()

			p.OnExecuteCommand(func(string, string, []string, string, []PipeOperation) (string, any, error) {
				return rpc.StatusDone, map[string]any{"alive": true}, nil
			})

			loopDone := make(chan error, 1)
			if typed {
				p.activateBridge()
				go func() { loopDone <- p.bridgeEventLoop(ctx) }()
			} else {
				go func() { loopDone <- p.eventLoop(ctx) }()
			}

			// 1. Panicking collector returns error without closing transport.
			_, err = conn.SendRuntimeInspect(ctx, rpc.RuntimeInspectInput{Kind: rpc.RuntimeInspectRuntimeStats})
			require.Error(t, err)
			require.Contains(t, err.Error(), "panicked")

			// 2. Expired deadline returns error without closing transport.
			_, err = conn.SendRuntimeInspect(ctx, rpc.RuntimeInspectInput{
				Kind:       rpc.RuntimeInspectRuntimeStats,
				DeadlineMS: time.Now().Add(-time.Second).UnixMilli(),
			})
			require.Error(t, err)

			// 3. Subsequent runtime-inspect succeeds on the exact same connection.
			out, err := conn.SendRuntimeInspect(ctx, rpc.RuntimeInspectInput{Kind: rpc.RuntimeInspectRuntimeStats})
			require.NoError(t, err)
			require.Equal(t, os.Getpid(), out.PID)

			// 4. Ordinary callbacks still work on the exact same connection.
			if typed {
				require.True(t, bridge.Ready())
				_, cmdErr := bridge.SendCallback(ctx, testMethodExecuteCommand, json.RawMessage(`{"command":"ping"}`))
				require.NoError(t, cmdErr)
			} else {
				answer, cmdErr := engine.mux.CallAnswer(ctx, testMethodExecuteCommand, map[string]any{"command": "ping"})
				require.NoError(t, cmdErr)
				for range answer.Records {
				}
				require.NoError(t, answer.Err())
			}

			cancel()
			<-loopDone
		})
	}

	t.Run("bridge-queue-saturation-preserves-bridge", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()

		p, _, bridge := newBridgedTestPair(t)
		entered, release := make(chan struct{}), make(chan struct{})
		var gateOnce atomic.Bool
		stop, err := p.startRuntimeInspectWorkerWithCollector(ctx, func(c context.Context, in rpc.RuntimeInspectInput) (*rpc.RuntimeInspectOutput, error) {
			if gateOnce.CompareAndSwap(false, true) {
				close(entered)
				select {
				case <-release:
				case <-c.Done():
					return nil, c.Err()
				}
			}
			return collectRuntimeInspect(c, in)
		})
		require.NoError(t, err)
		defer func() { require.NoError(t, stop()) }()
		bridge.SetReady()

		// Spawn 1 active + runtimeInspectQueueDepth queued + 1 overflow caller.
		// By pigeonhole principle, while the active collector is held on release,
		// at least one caller MUST fail immediately with errRuntimeInspectFull.
		total := runtimeInspectQueueDepth + 2
		errs := make(chan error, total)
		for range total {
			go func() {
				_, callErr := bridge.RuntimeInspect(ctx, rpc.RuntimeInspectInput{Kind: rpc.RuntimeInspectRuntimeStats})
				errs <- callErr
			}()
		}

		select {
		case <-entered:
		case <-ctx.Done():
			t.Fatal("first request never entered collector")
		}

		select {
		case fullErr := <-errs:
			require.Error(t, fullErr)
			require.True(t, strings.Contains(fullErr.Error(), errRuntimeInspectFull.Error()), "unexpected saturation error: %v", fullErr)
		case <-ctx.Done():
			t.Fatal("expected immediate saturation error when queue is full")
		}
		require.True(t, bridge.Ready())

		close(release)
		for range total - 1 {
			require.NoError(t, <-errs)
		}
	})

	t.Run("wire-queue-overflow-does-not-close-mux", func(t *testing.T) {
		p, _ := newTestPair(t)
		w := &runtimeInspectWorker{
			plugin:    p,
			collector: collectRuntimeInspect,
			requests:  make(chan runtimeInspectJob, 1),
			done:      make(chan struct{}),
		}
		req1 := &rpc.Request{ID: 1, Method: rpc.MethodRuntimeInspect}
		req2 := &rpc.Request{ID: 2, Method: rpc.MethodRuntimeInspect}
		require.NoError(t, w.admitWire(req1))
		// Overflow must return nil so MuxConn.readLoop does not close the socket.
		require.NoError(t, w.admitWire(req2))
		w.stopAdmission()
		require.NoError(t, w.admitWire(req2))
	})
}

// TestRuntimeInspectTruncationAndLogFiltering verifies bounded goroutine stacks,
// bounded profile buffers, and log-recent filtering.
//
// VALIDATES: AC-4, goroutine and profile buffers set Truncated when capped and
// stay within wire bounds; log-recent respects count/level/component filters.
// PREVENTS: oversized runtime inspection replies exceeding rpc.MaxMessageSize.
func TestRuntimeInspectTruncationAndLogFiltering(t *testing.T) {
	stacks, truncated := collectRuntimeGoroutines(64)
	require.True(t, truncated)
	require.Len(t, stacks, 64)

	profOut, err := collectRuntimeSnapshotProfile(rpc.RuntimeInspectProfileHeap, 64)
	require.NoError(t, err)
	require.True(t, profOut.Truncated)
	decoded, err := base64.StdEncoding.DecodeString(profOut.ProfileData)
	require.NoError(t, err)
	require.LessOrEqual(t, len(decoded), 64)
	require.NoError(t, profOut.Validate(rpc.RuntimeInspectInput{
		Kind:        rpc.RuntimeInspectProfile,
		ProfileType: rpc.RuntimeInspectProfileHeap,
	}))

	var buf cappedProfileBuffer
	buf.maxBytes = 8
	n, err := buf.Write([]byte("12345678"))
	require.NoError(t, err)
	require.Equal(t, 8, n)
	require.False(t, buf.truncated)

	n, err = buf.Write([]byte("90"))
	require.NoError(t, err)
	require.Equal(t, 2, n)
	require.True(t, buf.truncated)
	require.Equal(t, "12345678", buf.buf.String())

	const comp = "diag3-log-filter"
	slogutil.Logger(comp).Error("diag3 err one")
	slogutil.Logger(comp).Error("diag3 err two")
	slogutil.Logger(comp).Info("diag3 info one")

	logOut, err := collectRuntimeLogRecent(rpc.RuntimeInspectInput{
		Kind:         rpc.RuntimeInspectLogRecent,
		LogLevel:     "error",
		LogComponent: comp,
		LogCount:     1,
	})
	require.NoError(t, err)
	require.Len(t, logOut.LogEntries, 1)
	require.Equal(t, "error", logOut.LogEntries[0].Level)
	require.Equal(t, comp, logOut.LogEntries[0].Component)

	_, err = collectRuntimeLogRecent(rpc.RuntimeInspectInput{
		Kind:     rpc.RuntimeInspectLogRecent,
		LogLevel: "not-a-level",
	})
	require.Error(t, err)
}
