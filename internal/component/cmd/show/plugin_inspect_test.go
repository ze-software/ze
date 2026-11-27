// VALIDATES: command-owned `| plugin <name>` pipe filter registration, refusal handling,
// RPC response formatting, `| summary` composition, and host-side fallback across
// show system goroutines, show system profile, show runtime memory, and show system memory.
// PREVENTS: pipe filter registration panics or omissions, broken | plugin + | summary
// composition, missing opererr refusals, or malformed fallback/procfs payloads.

package show

import (
	"bytes"
	"context"
	"encoding/base64"
	"os"
	"runtime"
	"runtime/pprof"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/procfs"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func withStubInspectPlugin(
	t *testing.T,
	fn func(context.Context, string, rpc.RuntimeInspectInput) (*pluginserver.PluginInspectResult, *opererr.Error),
) {
	t.Helper()
	prev := inspectPluginFunc
	inspectPluginFunc = func(_ *pluginserver.Server, ctx context.Context, name string, input rpc.RuntimeInspectInput) (*pluginserver.PluginInspectResult, *opererr.Error) {
		return fn(ctx, name, input)
	}
	t.Cleanup(func() { inspectPluginFunc = prev })
}

func withStubInspectPluginHost(
	t *testing.T,
	fn func(string) (*pluginserver.PluginHostInspectResult, *opererr.Error),
) {
	t.Helper()
	prev := inspectPluginHostFunc
	inspectPluginHostFunc = func(_ *pluginserver.Server, name string) (*pluginserver.PluginHostInspectResult, *opererr.Error) {
		return fn(name)
	}
	t.Cleanup(func() { inspectPluginHostFunc = prev })
}

func TestPluginPipeFiltersRegisteredOnShowCommands(t *testing.T) {
	for _, cmdPath := range []string{
		"show system goroutines",
		"show system profile",
		"show runtime memory",
		"show system memory",
	} {
		filters := command.PipeFiltersForCommand(cmdPath)
		foundPlugin := false
		for _, f := range filters {
			if f.Name == pipeOpPlugin && f.Argument == command.ArgText {
				foundPlugin = true
				break
			}
		}
		assert.True(t, foundPlugin, "%q missing | plugin (ArgText) filter in %+v", cmdPath, filters)
	}

	// Verify show system profile keeps both summary and plugin registered together.
	profileFilters := command.PipeFiltersForCommand("show system profile")
	foundSummary := false
	for _, f := range profileFilters {
		if f.Name == pipeOpSummary && f.Argument == command.ArgNone {
			foundSummary = true
			break
		}
	}
	assert.True(t, foundSummary, "show system profile missing | summary filter alongside | plugin")
}

func TestPluginPipeRefusals(t *testing.T) {
	srv, err := pluginserver.NewServer(nil, nil)
	require.NoError(t, err)
	liveCtx := &pluginserver.CommandContext{Server: srv}

	handlers := []struct {
		name    string
		handler pluginserver.Handler
		args    []string
	}{
		{"goroutines", handleShowSystemGoroutines, []string{"summary"}},
		{"profile", handleShowSystemProfile, []string{"type", "heap"}},
		{"runtime-memory", handleShowSystemMemory, nil},
	}
	if runtime.GOOS == "linux" {
		handlers = append(handlers, struct {
			name    string
			handler pluginserver.Handler
			args    []string
		}{"system-memory", handleShowSystemMemoryMap, nil})
	}

	for _, h := range handlers {
		t.Run(h.name+"/duplicate-plugin", func(t *testing.T) {
			args := commandtest.Args(h.args...).WithPipeOperations([]command.PipeOperation{
				{Name: pipeOpPlugin, Args: []string{"a"}},
				{Name: pipeOpPlugin, Args: []string{"b"}},
			})
			resp, err := h.handler(liveCtx, args)
			require.NoError(t, err)
			require.NotNil(t, resp)
			assert.Equal(t, plugin.StatusError, resp.Status)
			assert.Contains(t, resp.Error, "duplicate | plugin")
		})

		t.Run(h.name+"/blank-plugin", func(t *testing.T) {
			args := commandtest.Args(h.args...).WithPipeOperations([]command.PipeOperation{
				{Name: pipeOpPlugin, Args: []string{"   "}},
			})
			resp, err := h.handler(liveCtx, args)
			require.NoError(t, err)
			require.NotNil(t, resp)
			assert.Equal(t, plugin.StatusError, resp.Status)
			assert.Contains(t, resp.Error, "| plugin requires a plugin name")
		})

		t.Run(h.name+"/missing-arg-plugin", func(t *testing.T) {
			args := commandtest.Args(h.args...).WithPipeOperations([]command.PipeOperation{
				{Name: pipeOpPlugin},
			})
			resp, err := h.handler(liveCtx, args)
			require.NoError(t, err)
			require.NotNil(t, resp)
			assert.Equal(t, plugin.StatusError, resp.Status)
			assert.Contains(t, resp.Error, "| plugin requires a plugin name")
		})

		t.Run(h.name+"/nil-server", func(t *testing.T) {
			args := commandtest.Args(h.args...).WithPipeOperations([]command.PipeOperation{
				{Name: pipeOpPlugin, Args: []string{"my-ddos"}},
			})
			resp, err := h.handler(nil, args)
			require.NoError(t, err)
			require.NotNil(t, resp)
			assert.Equal(t, plugin.StatusError, resp.Status)
			assert.Contains(t, resp.Error, "ze show system subsystem list")
		})

		t.Run(h.name+"/unknown-plugin", func(t *testing.T) {
			args := commandtest.Args(h.args...).WithPipeOperations([]command.PipeOperation{
				{Name: pipeOpPlugin, Args: []string{"no-such-plugin"}},
			})
			resp, err := h.handler(liveCtx, args)
			require.NoError(t, err)
			require.NotNil(t, resp)
			assert.Equal(t, plugin.StatusError, resp.Status)
			assert.Contains(t, resp.Error, "no-such-plugin")
			assert.Contains(t, resp.Error, "ze show system subsystem list")
		})
	}
}

func TestShowSystemGoroutinesPluginPipe(t *testing.T) {
	srv := &pluginserver.Server{}
	cmdCtx := &pluginserver.CommandContext{Server: srv}
	sampleStacks := "goroutine 1 [running]:\nmain.main()\n\t/app/main.go:10 +0x1\n\n" +
		"goroutine 7 [chan receive, 3 minutes]:\nworker.loop()\n\t/app/worker.go:42 +0x2\n"

	withStubInspectPlugin(t, func(_ context.Context, name string, input rpc.RuntimeInspectInput) (*pluginserver.PluginInspectResult, *opererr.Error) {
		assert.Equal(t, "my-ddos", name)
		assert.Equal(t, rpc.RuntimeInspectGoroutines, input.Kind)
		return &pluginserver.PluginInspectResult{
			Plugin:      name,
			ProcessMode: "external",
			PID:         9123,
			Output: &rpc.RuntimeInspectOutput{
				Kind:            rpc.RuntimeInspectGoroutines,
				PID:             9123,
				Truncated:       true,
				GoroutineStacks: sampleStacks,
			},
		}, nil
	})

	pipe := []command.PipeOperation{{Name: pipeOpPlugin, Args: []string{"my-ddos"}}}

	t.Run("summary", func(t *testing.T) {
		resp, err := handleShowSystemGoroutines(cmdCtx, commandtest.Args("summary").WithPipeOperations(pipe))
		require.NoError(t, err)
		require.Equal(t, plugin.StatusDone, resp.Status)
		data, ok := resp.Data.(plugin.Map)
		require.True(t, ok)
		assert.Equal(t, "summary", data["mode"])
		assert.Equal(t, 2, data["total"])
		assert.Equal(t, "my-ddos", data[keyPlugin])
		assert.Equal(t, "external", data[keyProcessMode])
		assert.Equal(t, 9123, data[keyPID])
		assert.Equal(t, true, data[triageKeyTruncated])
	})

	t.Run("blocked", func(t *testing.T) {
		resp, err := handleShowSystemGoroutines(cmdCtx, commandtest.Args("blocked").WithPipeOperations(pipe))
		require.NoError(t, err)
		require.Equal(t, plugin.StatusDone, resp.Status)
		data, ok := resp.Data.(plugin.Map)
		require.True(t, ok)
		assert.Equal(t, "blocked", data["mode"])
		assert.Equal(t, 1, data["blocked-count"])
		assert.Equal(t, "my-ddos", data[keyPlugin])
	})

	t.Run("full", func(t *testing.T) {
		resp, err := handleShowSystemGoroutines(cmdCtx, commandtest.Args("full").WithPipeOperations(pipe))
		require.NoError(t, err)
		require.Equal(t, plugin.StatusDone, resp.Status)
		data, ok := resp.Data.(plugin.Map)
		require.True(t, ok)
		assert.Equal(t, "full", data["mode"])
		assert.Equal(t, sampleStacks, data["stacks"])
		assert.Equal(t, 9123, data[keyPID])
	})

	t.Run("fallback", func(t *testing.T) {
		fdCount := 14
		withStubInspectPlugin(t, func(_ context.Context, name string, _ rpc.RuntimeInspectInput) (*pluginserver.PluginInspectResult, *opererr.Error) {
			return &pluginserver.PluginInspectResult{
				Plugin:         name,
				ProcessMode:    "external",
				PID:            9123,
				Fallback:       true,
				FallbackReason: "timeout",
				RPCError:       "context deadline exceeded",
				Host: &process.HostProcessSnapshot{
					PID:        9123,
					Running:    true,
					Stage:      "Running",
					StderrTail: []string{"deadlock suspected"},
					Proc: &procfs.ProcessInspect{
						State:   "S (sleeping)",
						Threads: 6,
						VmRSSKB: 12345,
						Wchan:   "futex_wait_queue",
						FDCount: &fdCount,
					},
				},
			}, nil
		})

		resp, err := handleShowSystemGoroutines(cmdCtx, commandtest.Args("blocked").WithPipeOperations(pipe))
		require.NoError(t, err)
		require.Equal(t, plugin.StatusDone, resp.Status)
		data, ok := resp.Data.(plugin.Map)
		require.True(t, ok)
		assert.Equal(t, "blocked", data["mode"])
		assert.Equal(t, true, data[keyFallback])
		assert.Equal(t, "timeout", data[keyFallbackReason])
		assert.Equal(t, "context deadline exceeded", data[keyRPCError])
		assert.Equal(t, true, data["running"])
		assert.Equal(t, "Running", data["stage"])
		assert.Equal(t, []string{"deadlock suspected"}, data[keyStderrTail])
		assert.NotNil(t, data[keyHost])
	})
}

func TestShowSystemProfilePluginPipe(t *testing.T) {
	srv := &pluginserver.Server{}
	cmdCtx := &pluginserver.CommandContext{Server: srv}

	var heapBuf bytes.Buffer
	require.NoError(t, pprof.Lookup("heap").WriteTo(&heapBuf, 0))
	heapB64 := base64.StdEncoding.EncodeToString(heapBuf.Bytes())
	mutexRate := 10

	var lastInput rpc.RuntimeInspectInput
	withStubInspectPlugin(t, func(_ context.Context, name string, input rpc.RuntimeInspectInput) (*pluginserver.PluginInspectResult, *opererr.Error) {
		lastInput = input
		out := &rpc.RuntimeInspectOutput{
			Kind:         rpc.RuntimeInspectProfile,
			PID:          7777,
			ProfileType:  input.ProfileType,
			ProfileCount: 42,
			ProfileData:  heapB64,
		}
		if input.ProfileType == profileTypeMutex {
			out.SamplingRate = &mutexRate
		}
		if input.ProfileType == profileTypeAllocs {
			out.Truncated = true
		}
		return &pluginserver.PluginInspectResult{
			Plugin:      name,
			ProcessMode: "external",
			PID:         7777,
			Output:      out,
		}, nil
	})

	t.Run("bare-mutex-preserves-sampling-rate", func(t *testing.T) {
		args := commandtest.Args("type", "mutex").WithPipeOperations([]command.PipeOperation{
			{Name: pipeOpPlugin, Args: []string{"my-ddos"}},
		})
		resp, err := handleShowSystemProfile(cmdCtx, args)
		require.NoError(t, err)
		require.Equal(t, plugin.StatusDone, resp.Status)
		data, ok := resp.Data.(plugin.Map)
		require.True(t, ok)
		assert.Equal(t, "mutex", data[keyType])
		assert.Equal(t, profileFormatPprofBase64, data[keyFormat])
		assert.Equal(t, heapB64, data[keyData])
		assert.Equal(t, 42, data[keyCount])
		assert.Equal(t, 10, data[keySamplingRate])
		assert.Equal(t, "my-ddos", data[keyPlugin])
		assert.Equal(t, "external", data[keyProcessMode])
		assert.Equal(t, 7777, data[keyPID])
	})

	t.Run("composed-plugin-then-summary", func(t *testing.T) {
		args := commandtest.Args("type", "heap").WithPipeOperations([]command.PipeOperation{
			{Name: pipeOpPlugin, Args: []string{"my-ddos"}},
			{Name: pipeOpSummary},
		})
		resp, err := handleShowSystemProfile(cmdCtx, args)
		require.NoError(t, err)
		require.Equal(t, plugin.StatusDone, resp.Status)
		data, ok := resp.Data.(plugin.Map)
		require.True(t, ok)
		assertProfileSummaryShape(t, data, "heap")
		assert.Equal(t, 42, data["profile-count"])
		assert.Equal(t, "my-ddos", data[keyPlugin])
		assert.Equal(t, "external", data[keyProcessMode])
		assert.Equal(t, 7777, data[keyPID])
	})

	t.Run("composed-summary-then-plugin-and-sampling-rate", func(t *testing.T) {
		args := commandtest.Args("type", "mutex").WithPipeOperations([]command.PipeOperation{
			{Name: pipeOpSummary},
			{Name: pipeOpPlugin, Args: []string{"my-ddos"}},
		})
		resp, err := handleShowSystemProfile(cmdCtx, args)
		require.NoError(t, err)
		require.Equal(t, plugin.StatusDone, resp.Status)
		data, ok := resp.Data.(plugin.Map)
		require.True(t, ok)
		assertProfileSummaryShape(t, data, "mutex")
		assert.Equal(t, 10, data[keySamplingRate])
		assert.Equal(t, "my-ddos", data[keyPlugin])
	})

	t.Run("cpu-duration-propagated", func(t *testing.T) {
		args := commandtest.Args("type", "cpu", "duration", "2s").WithPipeOperations([]command.PipeOperation{
			{Name: pipeOpPlugin, Args: []string{"my-ddos"}},
			{Name: pipeOpSummary},
		})
		resp, err := handleShowSystemProfile(cmdCtx, args)
		require.NoError(t, err)
		require.Equal(t, plugin.StatusDone, resp.Status)
		assert.Equal(t, int64(2000), lastInput.DurationMS)
		data, ok := resp.Data.(plugin.Map)
		require.True(t, ok)
		assert.Equal(t, "2s", data["duration"])
		assert.Equal(t, "my-ddos", data[keyPlugin])
	})

	t.Run("truncated-flag-propagated", func(t *testing.T) {
		args := commandtest.Args("type", "allocs").WithPipeOperations([]command.PipeOperation{
			{Name: pipeOpPlugin, Args: []string{"my-ddos"}},
		})
		resp, err := handleShowSystemProfile(cmdCtx, args)
		require.NoError(t, err)
		require.Equal(t, plugin.StatusDone, resp.Status)
		data, ok := resp.Data.(plugin.Map)
		require.True(t, ok)
		assert.Equal(t, true, data[triageKeyTruncated])
	})

	t.Run("fallback", func(t *testing.T) {
		withStubInspectPlugin(t, func(_ context.Context, name string, _ rpc.RuntimeInspectInput) (*pluginserver.PluginInspectResult, *opererr.Error) {
			return &pluginserver.PluginInspectResult{
				Plugin:         name,
				ProcessMode:    "external",
				PID:            7777,
				Fallback:       true,
				FallbackReason: "unsupported",
				RPCError:       "unknown method",
				Host: &process.HostProcessSnapshot{
					PID:        7777,
					Running:    true,
					Stage:      "Running",
					ProcError:  "procfs unavailable",
					StderrTail: []string{},
				},
			}, nil
		})
		args := commandtest.Args("type", "heap").WithPipeOperations([]command.PipeOperation{
			{Name: pipeOpPlugin, Args: []string{"ext-py"}},
			{Name: pipeOpSummary},
		})
		resp, err := handleShowSystemProfile(cmdCtx, args)
		require.NoError(t, err)
		require.Equal(t, plugin.StatusDone, resp.Status)
		data, ok := resp.Data.(plugin.Map)
		require.True(t, ok)
		assert.Equal(t, "heap", data[keyType])
		assert.Equal(t, true, data[keyFallback])
		assert.Equal(t, "unsupported", data[keyFallbackReason])
		assert.Equal(t, "procfs unavailable", data[keyProcfsError])
	})
}

func TestShowRuntimeMemoryPluginPipe(t *testing.T) {
	srv := &pluginserver.Server{}
	cmdCtx := &pluginserver.CommandContext{Server: srv}
	pipe := []command.PipeOperation{{Name: pipeOpPlugin, Args: []string{"my-ddos"}}}

	t.Run("rpc-success", func(t *testing.T) {
		withStubInspectPlugin(t, func(_ context.Context, name string, input rpc.RuntimeInspectInput) (*pluginserver.PluginInspectResult, *opererr.Error) {
			assert.Equal(t, rpc.RuntimeInspectRuntimeStats, input.Kind)
			return &pluginserver.PluginInspectResult{
				Plugin:      name,
				ProcessMode: "external",
				PID:         8888,
				Output: &rpc.RuntimeInspectOutput{
					Kind: rpc.RuntimeInspectRuntimeStats,
					PID:  8888,
					RuntimeStats: &rpc.RuntimeInspectStats{
						Alloc:         1000,
						TotalAlloc:    2000,
						Sys:           3000,
						HeapAlloc:     1100,
						HeapSys:       2200,
						HeapInuse:     1500,
						HeapObjects:   50,
						StackInuse:    4096,
						NumGC:         3,
						GCCPUPct:      0.25,
						NumGoroutines: 12,
						NumCPU:        8,
						MaxProcs:      8,
						GoVersion:     runtime.Version(),
					},
				},
			}, nil
		})

		resp, err := handleShowSystemMemory(cmdCtx, commandtest.Args().WithPipeOperations(pipe))
		require.NoError(t, err)
		require.Equal(t, plugin.StatusDone, resp.Status)
		data, ok := resp.Data.(plugin.Map)
		require.True(t, ok)
		assert.Equal(t, uint64(1000), data["alloc"])
		assert.Equal(t, uint64(1500), data["heap-in-use"])
		assert.Equal(t, uint32(3), data["num-gc"])
		assert.Equal(t, 12, data["num-goroutines"])
		assert.Equal(t, 8, data["max-procs"])
		assert.Equal(t, runtime.Version(), data["go-version"])
		assert.Equal(t, "my-ddos", data[keyPlugin])
		assert.Equal(t, "external", data[keyProcessMode])
		assert.Equal(t, 8888, data[keyPID])
	})

	t.Run("fallback", func(t *testing.T) {
		withStubInspectPlugin(t, func(_ context.Context, name string, _ rpc.RuntimeInspectInput) (*pluginserver.PluginInspectResult, *opererr.Error) {
			return &pluginserver.PluginInspectResult{
				Plugin:         name,
				ProcessMode:    "external",
				PID:            8888,
				Fallback:       true,
				FallbackReason: "transport-error",
				RPCError:       "plugin connection closed",
				Host: &process.HostProcessSnapshot{
					PID:        8888,
					Running:    true,
					Stage:      "Running",
					StderrTail: []string{"fatal: out of memory"},
				},
			}, nil
		})

		resp, err := handleShowSystemMemory(cmdCtx, commandtest.Args().WithPipeOperations(pipe))
		require.NoError(t, err)
		require.Equal(t, plugin.StatusDone, resp.Status)
		data, ok := resp.Data.(plugin.Map)
		require.True(t, ok)
		assert.Equal(t, true, data[keyFallback])
		assert.Equal(t, "transport-error", data[keyFallbackReason])
		assert.Equal(t, []string{"fatal: out of memory"}, data[keyStderrTail])
	})
}

func TestShowSystemMemoryMapPluginPipe(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("show system memory OS map requires Linux /proc")
	}

	srv := &pluginserver.Server{}
	cmdCtx := &pluginserver.CommandContext{Server: srv}

	t.Run("internal-shared-daemon-process", func(t *testing.T) {
		withStubInspectPluginHost(t, func(name string) (*pluginserver.PluginHostInspectResult, *opererr.Error) {
			return &pluginserver.PluginHostInspectResult{
				Plugin:      name,
				ProcessMode: "internal",
				Host:        process.HostProcessSnapshot{Running: true, Stage: "Running"},
			}, nil
		})

		args := commandtest.Args().WithPipeOperations([]command.PipeOperation{{Name: pipeOpPlugin, Args: []string{"bgp-rib"}}})
		resp, err := handleShowSystemMemoryMap(cmdCtx, args)
		require.NoError(t, err)
		require.Equal(t, plugin.StatusDone, resp.Status)
		data, ok := resp.Data.(plugin.Map)
		require.True(t, ok)
		assert.Equal(t, "bgp-rib", data[keyPlugin])
		assert.Equal(t, "internal", data[keyProcessMode])
		assert.Equal(t, os.Getpid(), data[keyPID])
		assert.Equal(t, true, data[keySharedDaemonProcess])
		assert.Positive(t, data["vm-rss-kb"])
	})

	t.Run("external-live-proc", func(t *testing.T) {
		fdCount := 21
		withStubInspectPluginHost(t, func(name string) (*pluginserver.PluginHostInspectResult, *opererr.Error) {
			return &pluginserver.PluginHostInspectResult{
				Plugin:      name,
				ProcessMode: "external",
				Host: process.HostProcessSnapshot{
					PID:     5555,
					Running: true,
					Stage:   "Running",
					Proc: &procfs.ProcessInspect{
						State:     "S (sleeping)",
						Threads:   5,
						VmRSSKB:   20480,
						VmSizeKB:  90000,
						VmSwapKB:  0,
						VmPeakKB:  95000,
						VmDataKB:  16000,
						VmStackKB: 132,
						Wchan:     "ep_poll",
						FDCount:   &fdCount,
					},
				},
			}, nil
		})

		args := commandtest.Args().WithPipeOperations([]command.PipeOperation{{Name: pipeOpPlugin, Args: []string{"my-ddos"}}})
		resp, err := handleShowSystemMemoryMap(cmdCtx, args)
		require.NoError(t, err)
		require.Equal(t, plugin.StatusDone, resp.Status)
		data, ok := resp.Data.(plugin.Map)
		require.True(t, ok)
		assert.Equal(t, "my-ddos", data[keyPlugin])
		assert.Equal(t, "external", data[keyProcessMode])
		assert.Equal(t, 5555, data[keyPID])
		assert.Equal(t, 20480, data["vm-rss-kb"])
		assert.Equal(t, 5, data["threads"])
		assert.Equal(t, "ep_poll", data["wchan"])
		assert.Equal(t, 21, data["fd-count"])
		_, hasShared := data[keySharedDaemonProcess]
		assert.False(t, hasShared)
	})

	t.Run("external-no-pid-refused", func(t *testing.T) {
		withStubInspectPluginHost(t, func(name string) (*pluginserver.PluginHostInspectResult, *opererr.Error) {
			return &pluginserver.PluginHostInspectResult{
				Plugin:      name,
				ProcessMode: "external",
				Host:        process.HostProcessSnapshot{PID: 0, Running: false},
			}, nil
		})

		args := commandtest.Args().WithPipeOperations([]command.PipeOperation{{Name: pipeOpPlugin, Args: []string{"dead-ext"}}})
		resp, err := handleShowSystemMemoryMap(cmdCtx, args)
		require.NoError(t, err)
		require.Equal(t, plugin.StatusError, resp.Status)
		assert.Contains(t, resp.Error, "has no running OS process")
	})

	t.Run("external-proc-error-refused", func(t *testing.T) {
		withStubInspectPluginHost(t, func(name string) (*pluginserver.PluginHostInspectResult, *opererr.Error) {
			return &pluginserver.PluginHostInspectResult{
				Plugin:      name,
				ProcessMode: "external",
				Host:        process.HostProcessSnapshot{PID: 5555, Running: true, ProcError: "open /proc/5555/status: no such file"},
			}, nil
		})

		args := commandtest.Args().WithPipeOperations([]command.PipeOperation{{Name: pipeOpPlugin, Args: []string{"gone-ext"}}})
		resp, err := handleShowSystemMemoryMap(cmdCtx, args)
		require.NoError(t, err)
		require.Equal(t, plugin.StatusError, resp.Status)
		assert.Contains(t, resp.Error, "cannot read /proc status")
	})
}
