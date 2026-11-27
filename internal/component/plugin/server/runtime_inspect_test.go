// VALIDATES: Server.InspectPlugin and Server.InspectPluginHost routing, per-process
// concurrency cap of 4, internal DirectBridge vs external MuxConn handling, and
// non-destructive external host fallback on timeout, unsupported method, and transport error.
// PREVENTS: diagnostic bursts overloading a plugin, leaked inspectInFlight map entries,
// internal plugin errors returning fabricated external fallbacks, or external fallbacks
// failing to surface /proc and stderr-tail diagnostics.

package server

import (
	"context"
	"errors"
	"math"
	"net"
	"os"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func setProcessExternalForTest(t *testing.T, proc *process.Process, pid int) {
	t.Helper()

	extField := reflect.ValueOf(proc).Elem().FieldByName("isExternal")
	require.True(t, extField.IsValid(), "process.isExternal field must exist")
	ext := (*atomic.Bool)(unsafe.Pointer(extField.UnsafeAddr())) //nolint:gosec // G103: test-only field wiring matching markProcessRunning
	ext.Store(true)

	if pid > 0 {
		require.LessOrEqual(t, pid, math.MaxInt32)
		pidField := reflect.ValueOf(proc).Elem().FieldByName("pid")
		require.True(t, pidField.IsValid(), "process.pid field must exist")
		p := (*atomic.Int32)(unsafe.Pointer(pidField.UnsafeAddr())) //nolint:gosec // G103: test-only field wiring matching markProcessRunning
		p.Store(int32(pid))                                         //nolint:gosec // G115: checked above
	}
}

func newServerWithProcess(t *testing.T, name string, proc *process.Process) *Server {
	t.Helper()
	pm := process.NewProcessManager(nil)
	require.NoError(t, pm.AddProcess(name, proc))
	s := &Server{}
	s.procManager.Store(pm)
	return s
}

func TestInspectPluginUnknownAndUnavailable(t *testing.T) {
	t.Run("nil-server", func(t *testing.T) {
		var s *Server
		res, opErr := s.InspectPlugin(t.Context(), "ddos", rpc.RuntimeInspectInput{Kind: rpc.RuntimeInspectRuntimeStats})
		assert.Nil(t, res)
		require.NotNil(t, opErr)
		assert.Contains(t, opErr.Error(), "ze show system subsystem list")

		hostRes, hostErr := s.InspectPluginHost("ddos")
		assert.Nil(t, hostRes)
		require.NotNil(t, hostErr)
		assert.Contains(t, hostErr.Error(), "ze show system subsystem list")
	})

	t.Run("no-process-manager", func(t *testing.T) {
		s := &Server{}
		res, opErr := s.InspectPlugin(t.Context(), "missing", rpc.RuntimeInspectInput{Kind: rpc.RuntimeInspectRuntimeStats})
		assert.Nil(t, res)
		require.NotNil(t, opErr)
		assert.Contains(t, opErr.Error(), `"missing"`)
		assert.Contains(t, opErr.Error(), "ze show system subsystem list")

		hostRes, hostErr := s.InspectPluginHost("missing")
		assert.Nil(t, hostRes)
		require.NotNil(t, hostErr)
		assert.Contains(t, hostErr.Error(), "ze show system subsystem list")
	})

	t.Run("unknown-endpoint", func(t *testing.T) {
		proc := process.NewProcess(plugin.PluginConfig{Name: "other", Internal: true})
		s := newServerWithProcess(t, "other", proc)

		res, opErr := s.InspectPlugin(t.Context(), "missing", rpc.RuntimeInspectInput{Kind: rpc.RuntimeInspectRuntimeStats})
		assert.Nil(t, res)
		require.NotNil(t, opErr)
		assert.Contains(t, opErr.Error(), `"missing"`)
		assert.Contains(t, opErr.Error(), "ze show system subsystem list")

		hostRes, hostErr := s.InspectPluginHost("missing")
		assert.Nil(t, hostRes)
		require.NotNil(t, hostErr)
		assert.Contains(t, hostErr.Error(), `"missing"`)
		assert.Contains(t, hostErr.Error(), "ze show system subsystem list")
	})
}

func TestInspectPluginInternalDirectBridgeSuccessAndFailure(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		engineSide, pluginSide := net.Pipe()
		t.Cleanup(func() {
			_ = engineSide.Close()
			_ = pluginSide.Close()
		})

		conn := ipc.NewPluginConn(engineSide, engineSide)
		bridge := rpc.NewDirectBridge()
		bridge.SetRuntimeInspect(func(_ context.Context, input rpc.RuntimeInspectInput) (*rpc.RuntimeInspectOutput, error) {
			return &rpc.RuntimeInspectOutput{
				Kind: input.Kind,
				PID:  os.Getpid(),
				RuntimeStats: &rpc.RuntimeInspectStats{
					Alloc:         4096,
					NumGoroutines: 7,
					NumCPU:        4,
					MaxProcs:      4,
					GoVersion:     runtime.Version(),
				},
			}, nil
		})
		bridge.SetReady()
		conn.SetBridge(bridge)

		proc := process.NewProcess(plugin.PluginConfig{Name: "int-plugin", Internal: true})
		markProcessRunning(t, proc)
		proc.SetStage(plugin.StageRunning)
		proc.SetConn(conn)
		s := newServerWithProcess(t, "int-plugin", proc)

		res, opErr := s.InspectPlugin(t.Context(), "int-plugin", rpc.RuntimeInspectInput{
			Kind: rpc.RuntimeInspectRuntimeStats,
		})
		require.Nil(t, opErr)
		require.NotNil(t, res)
		assert.Equal(t, "int-plugin", res.Plugin)
		assert.Equal(t, processModeInternal, res.ProcessMode)
		assert.Equal(t, os.Getpid(), res.PID)
		assert.False(t, res.Fallback)
		require.NotNil(t, res.Output)
		require.NotNil(t, res.Output.RuntimeStats)
		assert.Equal(t, uint64(4096), res.Output.RuntimeStats.Alloc)

		s.inspectMu.Lock()
		assert.Empty(t, s.inspectInFlight)
		s.inspectMu.Unlock()
	})

	t.Run("internal-error-returns-opererr-not-fallback", func(t *testing.T) {
		proc := process.NewProcess(plugin.PluginConfig{Name: "int-broken", Internal: true})
		markProcessRunning(t, proc)
		s := newServerWithProcess(t, "int-broken", proc)

		res, opErr := s.InspectPlugin(t.Context(), "int-broken", rpc.RuntimeInspectInput{
			Kind: rpc.RuntimeInspectGoroutines,
		})
		assert.Nil(t, res)
		require.NotNil(t, opErr)
		assert.Contains(t, opErr.Error(), `"int-broken"`)
		assert.Contains(t, opErr.Error(), "ze show system subsystem list")
	})
}

func TestInspectPluginExternalMuxSuccessAndFallbacks(t *testing.T) {
	t.Run("mux-success", func(t *testing.T) {
		engineSide, pluginSide := net.Pipe()
		t.Cleanup(func() {
			_ = engineSide.Close()
			_ = pluginSide.Close()
		})

		pluginMux := rpc.NewMuxConn(rpc.NewConn(pluginSide, pluginSide))
		t.Cleanup(func() { _ = pluginMux.Close() })
		unregister, regErr := pluginMux.RegisterIndependentRequestHandler(rpc.MethodRuntimeInspect, func(req *rpc.Request) error {
			go func() {
				_ = pluginMux.SendResult(t.Context(), req.ID, &rpc.RuntimeInspectOutput{
					Kind:            rpc.RuntimeInspectGoroutines,
					PID:             4242,
					GoroutineStacks: "goroutine 1 [running]:\nmain.main()\n",
				})
			}()
			return nil
		})
		require.NoError(t, regErr)
		t.Cleanup(unregister)

		proc := process.NewProcess(plugin.PluginConfig{Name: "ext-ok"})
		setProcessExternalForTest(t, proc, os.Getpid())
		markProcessRunning(t, proc)
		proc.SetStage(plugin.StageRunning)
		proc.SetConn(muxPluginConn(t, engineSide))
		s := newServerWithProcess(t, "ext-ok", proc)

		res, opErr := s.InspectPlugin(t.Context(), "ext-ok", rpc.RuntimeInspectInput{
			Kind: rpc.RuntimeInspectGoroutines,
		})
		require.Nil(t, opErr)
		require.NotNil(t, res)
		assert.Equal(t, "ext-ok", res.Plugin)
		assert.Equal(t, processModeExternal, res.ProcessMode)
		assert.Equal(t, 4242, res.PID)
		assert.False(t, res.Fallback)
		require.NotNil(t, res.Output)
		assert.Contains(t, res.Output.GoroutineStacks, "goroutine 1")
	})

	t.Run("fallback-timeout", func(t *testing.T) {
		engineSide, pluginSide := net.Pipe()
		t.Cleanup(func() {
			_ = engineSide.Close()
			_ = pluginSide.Close()
		})

		pluginMux := rpc.NewMuxConn(rpc.NewConn(pluginSide, pluginSide))
		t.Cleanup(func() { _ = pluginMux.Close() })
		// Accept the request frame and deliberately never reply to simulate a wedged runtime.
		unregister, regErr := pluginMux.RegisterIndependentRequestHandler(rpc.MethodRuntimeInspect, func(_ *rpc.Request) error {
			return nil
		})
		require.NoError(t, regErr)
		t.Cleanup(unregister)

		proc := process.NewProcess(plugin.PluginConfig{Name: "ext-wedged"})
		setProcessExternalForTest(t, proc, os.Getpid())
		markProcessRunning(t, proc)
		proc.SetStage(plugin.StageRunning)
		proc.SetConn(muxPluginConn(t, engineSide))
		s := newServerWithProcess(t, "ext-wedged", proc)

		ctx, cancel := context.WithTimeout(t.Context(), 80*time.Millisecond)
		defer cancel()

		res, opErr := s.InspectPlugin(ctx, "ext-wedged", rpc.RuntimeInspectInput{
			Kind: rpc.RuntimeInspectGoroutines,
		})
		require.Nil(t, opErr)
		require.NotNil(t, res)
		assert.True(t, res.Fallback)
		assert.Equal(t, fallbackReasonTimeout, res.FallbackReason)
		assert.Equal(t, processModeExternal, res.ProcessMode)
		assert.Equal(t, os.Getpid(), res.PID)
		assert.NotEmpty(t, res.RPCError)
		require.NotNil(t, res.Host)
		assert.True(t, res.Host.Running)
		assert.Equal(t, "Running", res.Host.Stage)
		assert.NotNil(t, res.Host.StderrTail)
		if runtime.GOOS == "linux" {
			require.NotNil(t, res.Host.Proc)
			assert.Positive(t, res.Host.Proc.Threads)
			assert.Positive(t, res.Host.Proc.VmRSSKB)
		}
	})

	t.Run("fallback-unsupported", func(t *testing.T) {
		engineSide, pluginSide := net.Pipe()
		t.Cleanup(func() {
			_ = engineSide.Close()
			_ = pluginSide.Close()
		})

		pluginMux := rpc.NewMuxConn(rpc.NewConn(pluginSide, pluginSide))
		t.Cleanup(func() { _ = pluginMux.Close() })
		unregister, regErr := pluginMux.RegisterIndependentRequestHandler(rpc.MethodRuntimeInspect, func(req *rpc.Request) error {
			go func() {
				_ = pluginMux.SendError(t.Context(), req.ID, "unknown method: "+rpc.MethodRuntimeInspect)
			}()
			return nil
		})
		require.NoError(t, regErr)
		t.Cleanup(unregister)

		proc := process.NewProcess(plugin.PluginConfig{Name: "ext-python"})
		setProcessExternalForTest(t, proc, os.Getpid())
		markProcessRunning(t, proc)
		proc.SetStage(plugin.StageRunning)
		proc.SetConn(muxPluginConn(t, engineSide))
		s := newServerWithProcess(t, "ext-python", proc)

		res, opErr := s.InspectPlugin(t.Context(), "ext-python", rpc.RuntimeInspectInput{
			Kind: rpc.RuntimeInspectRuntimeStats,
		})
		require.Nil(t, opErr)
		require.NotNil(t, res)
		assert.True(t, res.Fallback)
		assert.Equal(t, fallbackReasonUnsupported, res.FallbackReason)
		assert.Contains(t, strings.ToLower(res.RPCError), "unknown method")
		require.NotNil(t, res.Host)
		assert.Equal(t, os.Getpid(), res.Host.PID)
	})

	t.Run("fallback-transport-error-when-conn-nil", func(t *testing.T) {
		proc := process.NewProcess(plugin.PluginConfig{Name: "ext-noconn"})
		setProcessExternalForTest(t, proc, os.Getpid())
		markProcessRunning(t, proc)
		proc.SetStage(plugin.StageRunning)
		s := newServerWithProcess(t, "ext-noconn", proc)

		res, opErr := s.InspectPlugin(t.Context(), "ext-noconn", rpc.RuntimeInspectInput{
			Kind: rpc.RuntimeInspectLogRecent,
		})
		require.Nil(t, opErr)
		require.NotNil(t, res)
		assert.True(t, res.Fallback)
		assert.Equal(t, fallbackReasonTransportError, res.FallbackReason)
		assert.Contains(t, res.RPCError, ErrPluginConnectionClosed.Error())
		require.NotNil(t, res.Host)
	})
}

func TestInspectPluginConcurrencyCap(t *testing.T) {
	engineSide, pluginSide := net.Pipe()
	t.Cleanup(func() {
		_ = engineSide.Close()
		_ = pluginSide.Close()
	})

	started := make(chan struct{}, runtimeInspectMaxInFlight)
	release := make(chan struct{})

	conn := ipc.NewPluginConn(engineSide, engineSide)
	bridge := rpc.NewDirectBridge()
	bridge.SetRuntimeInspect(func(ctx context.Context, input rpc.RuntimeInspectInput) (*rpc.RuntimeInspectOutput, error) {
		started <- struct{}{}
		select {
		case <-release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		return &rpc.RuntimeInspectOutput{
			Kind: input.Kind,
			PID:  os.Getpid(),
			RuntimeStats: &rpc.RuntimeInspectStats{
				NumGoroutines: 1,
				NumCPU:        1,
				MaxProcs:      1,
				GoVersion:     runtime.Version(),
			},
		}, nil
	})
	bridge.SetReady()
	conn.SetBridge(bridge)

	proc := process.NewProcess(plugin.PluginConfig{Name: "busy-plugin", Internal: true})
	markProcessRunning(t, proc)
	proc.SetStage(plugin.StageRunning)
	proc.SetConn(conn)
	s := newServerWithProcess(t, "busy-plugin", proc)

	var wg sync.WaitGroup
	errs := make(chan *opererr.Error, runtimeInspectMaxInFlight)
	for range runtimeInspectMaxInFlight {
		wg.Go(func() {
			_, opErr := s.InspectPlugin(t.Context(), "busy-plugin", rpc.RuntimeInspectInput{
				Kind: rpc.RuntimeInspectRuntimeStats,
			})
			errs <- opErr
		})
	}

	// Wait until all 4 inspection slots are actively held.
	for range runtimeInspectMaxInFlight {
		select {
		case <-started:
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for in-flight inspections to start")
		}
	}

	// The 5th concurrent inspection MUST be refused immediately.
	fifthRes, fifthErr := s.InspectPlugin(t.Context(), "busy-plugin", rpc.RuntimeInspectInput{
		Kind: rpc.RuntimeInspectRuntimeStats,
	})
	assert.Nil(t, fifthRes)
	require.NotNil(t, fifthErr)
	assert.Contains(t, fifthErr.Error(), "busy")

	close(release)
	wg.Wait()
	close(errs)
	for opErr := range errs {
		assert.Nil(t, opErr)
	}

	// Map entry MUST be deleted once in-flight count returns to 0.
	s.inspectMu.Lock()
	assert.Empty(t, s.inspectInFlight)
	s.inspectMu.Unlock()

	// Subsequent inspection succeeds cleanly.
	afterRes, afterErr := s.InspectPlugin(t.Context(), "busy-plugin", rpc.RuntimeInspectInput{
		Kind: rpc.RuntimeInspectRuntimeStats,
	})
	require.Nil(t, afterErr)
	require.NotNil(t, afterRes)
}

func TestInspectPluginHost(t *testing.T) {
	t.Run("internal", func(t *testing.T) {
		proc := process.NewProcess(plugin.PluginConfig{Name: "int-host", Internal: true})
		markProcessRunning(t, proc)
		proc.SetStage(plugin.StageRunning)
		s := newServerWithProcess(t, "int-host", proc)

		res, opErr := s.InspectPluginHost("int-host")
		require.Nil(t, opErr)
		require.NotNil(t, res)
		assert.Equal(t, "int-host", res.Plugin)
		assert.Equal(t, processModeInternal, res.ProcessMode)
		assert.Equal(t, 0, res.Host.PID)
	})

	t.Run("external", func(t *testing.T) {
		proc := process.NewProcess(plugin.PluginConfig{Name: "ext-host"})
		setProcessExternalForTest(t, proc, os.Getpid())
		markProcessRunning(t, proc)
		proc.SetStage(plugin.StageRunning)
		s := newServerWithProcess(t, "ext-host", proc)

		res, opErr := s.InspectPluginHost("ext-host")
		require.Nil(t, opErr)
		require.NotNil(t, res)
		assert.Equal(t, "ext-host", res.Plugin)
		assert.Equal(t, processModeExternal, res.ProcessMode)
		assert.Equal(t, os.Getpid(), res.Host.PID)
		if runtime.GOOS == "linux" {
			require.NotNil(t, res.Host.Proc)
			assert.Positive(t, res.Host.Proc.VmRSSKB)
		}
	})
}

func TestInspectPluginExternalAppAndValidationErrorsDoNotFallback(t *testing.T) {
	t.Run("invalid-input-refused-before-dispatch", func(t *testing.T) {
		proc := process.NewProcess(plugin.PluginConfig{Name: "ext-invalid"})
		setProcessExternalForTest(t, proc, os.Getpid())
		markProcessRunning(t, proc)
		proc.SetStage(plugin.StageRunning)
		s := newServerWithProcess(t, "ext-invalid", proc)

		res, opErr := s.InspectPlugin(t.Context(), "ext-invalid", rpc.RuntimeInspectInput{
			Kind: "bogus-kind",
		})
		require.Nil(t, res)
		require.NotNil(t, opErr)
		assert.Contains(t, opErr.Error(), "invalid runtime inspection request")
	})

	t.Run("live-external-rpc-application-error-not-swallowed", func(t *testing.T) {
		clientSide, pluginSide := net.Pipe()
		t.Cleanup(func() {
			clientSide.Close() //nolint:errcheck // test cleanup
			pluginSide.Close() //nolint:errcheck // test cleanup
		})

		clientMux := rpc.NewMuxConn(rpc.NewConn(clientSide, clientSide))
		pluginMux := rpc.NewMuxConn(rpc.NewConn(pluginSide, pluginSide))
		t.Cleanup(func() {
			clientMux.Close() //nolint:errcheck // test cleanup
			pluginMux.Close() //nolint:errcheck // test cleanup
		})

		remove, err := pluginMux.RegisterIndependentRequestHandler(rpc.MethodRuntimeInspect, func(req *rpc.Request) error {
			go func() {
				_ = pluginMux.SendError(t.Context(), req.ID, "CPU profile already in progress")
			}()
			return nil
		})
		require.NoError(t, err)
		t.Cleanup(remove)

		proc := process.NewProcess(plugin.PluginConfig{Name: "ext-busy-cpu"})
		setProcessExternalForTest(t, proc, os.Getpid())
		markProcessRunning(t, proc)
		proc.SetStage(plugin.StageRunning)
		proc.SetConn(ipc.NewMuxPluginConn(clientMux))
		s := newServerWithProcess(t, "ext-busy-cpu", proc)

		res, opErr := s.InspectPlugin(t.Context(), "ext-busy-cpu", rpc.RuntimeInspectInput{
			Kind:        rpc.RuntimeInspectProfile,
			ProfileType: rpc.RuntimeInspectProfileCPU,
			DurationMS:  1000,
		})
		require.Nil(t, res)
		require.NotNil(t, opErr)
		assert.Contains(t, opErr.Error(), "CPU profile already in progress")
		var callErr *rpc.RPCCallError
		assert.ErrorAs(t, opErr, &callErr)
	})
}

func TestClassifyInspectFallback(t *testing.T) {
	assert.Equal(t, fallbackReasonTransportError, classifyInspectFallback(nil))
	assert.Equal(t, fallbackReasonTimeout, classifyInspectFallback(context.DeadlineExceeded))
	assert.Equal(t, fallbackReasonTimeout, classifyInspectFallback(errors.New("i/o timeout")))
	assert.Equal(t, fallbackReasonTimeout, classifyInspectFallback(errors.New("request deadline exceeded")))
	assert.Equal(t, fallbackReasonUnsupported, classifyInspectFallback(errors.New("unknown method: runtime-inspect")))
	assert.Equal(t, fallbackReasonUnsupported, classifyInspectFallback(errors.New("operation not supported")))
	assert.Equal(t, fallbackReasonUnsupported, classifyInspectFallback(errors.New("unsupported RPC")))
	assert.Equal(t, fallbackReasonTransportError, classifyInspectFallback(errors.New("connection reset by peer")))
}
