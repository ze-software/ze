// Design: docs/architecture/diagnostics/procfs-diagnostics.md -- command-owned | plugin <name> pipe filter and inspection response builders
// Related: goroutines.go -- show system goroutines | plugin <name>
// Related: profile.go -- show system profile | plugin <name> [| summary]
// Related: system.go -- show runtime memory | plugin <name>
// Related: memory_map_linux.go -- show system memory | plugin <name>

package show

import (
	"context"
	"encoding/base64"
	"maps"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const (
	pipeOpPlugin           = "plugin"
	keyPlugin              = "plugin"
	keyProcessMode         = "process-mode"
	keyPID                 = "pid"
	keyFallback            = "fallback"
	keyFallbackReason      = "fallback-reason"
	keyRPCError            = "rpc-error"
	keyStderrTail          = "stderr-tail"
	keyHost                = "host"
	keyProcfsError         = "procfs-error"
	keySharedDaemonProcess = "shared-daemon-process"
	keyMode                = "mode"
	keyState               = "state"
	keySys                 = "sys"
	keyHeapAlloc           = "heap-alloc"
	keyHeapInUse           = "heap-in-use"
	keyNumGC               = "num-gc"
	keyNumGoroutines       = "num-goroutines"
	keyNumCPU              = "num-cpu"
	keyMaxProcs            = "max-procs"
)

// pluginPipeFilter declares the command-owned `| plugin <name>` pipe operation
// shared by show system goroutines, show system profile, show runtime memory,
// and show system memory. profile.go registers it alongside `| summary` in a
// single RegisterPipeFilters call for "show system profile".
var pluginPipeFilter = command.PipeFilter{
	Name:        pipeOpPlugin,
	Description: "Inspect a running plugin process by endpoint name instead of the daemon",
	Argument:    command.ArgText,
}

// inspectPluginFunc routes runtime inspection through Server.InspectPlugin.
// It is a package variable so unit tests in this package can exercise external
// and fallback response formatting deterministically; never written outside tests.
var inspectPluginFunc = func(
	s *pluginserver.Server,
	ctx context.Context,
	name string,
	input rpc.RuntimeInspectInput,
) (*pluginserver.PluginInspectResult, *opererr.Error) {
	return s.InspectPlugin(ctx, name, input)
}

// inspectPluginHostFunc routes host-side OS memory inspection through
// Server.InspectPluginHost. It is a package variable so unit tests in this
// package can exercise external and error paths deterministically; never
// written outside tests.
var inspectPluginHostFunc = func(s *pluginserver.Server, name string) (*pluginserver.PluginHostInspectResult, *opererr.Error) {
	return s.InspectPluginHost(name)
}

func extractPluginPipeTarget(validated command.ValidatedArgs, example string) (name string, present bool, refusal *plugin.Response) {
	return extractSingleTextPipeTarget(
		validated,
		pipeOpPlugin,
		"duplicate | plugin pipe operation",
		"only one | plugin selector may be given on a command line",
		"| plugin requires a plugin name",
		"an endpoint name must follow | plugin",
		"| plugin takes exactly one plugin name",
		"only one endpoint name may follow | plugin",
		example,
	)
}

func pluginServerUnavailableResponse() *plugin.Response {
	return plugin.OperatorErrorResponse(opererr.New(
		"plugin inspection is unavailable",
		"the daemon plugin server is not running in this context",
		"ze show system subsystem list",
	))
}

func pluginInspectServer(ctx *pluginserver.CommandContext) (*pluginserver.Server, *plugin.Response) {
	if ctx == nil {
		return nil, pluginServerUnavailableResponse()
	}
	if ctx.Server == nil {
		return nil, pluginServerUnavailableResponse()
	}
	return ctx.Server, nil
}

func pluginInspectFallbackResponse(res *pluginserver.PluginInspectResult, extra map[string]any) *plugin.Response {
	data := make(map[string]any, len(extra)+10)
	maps.Copy(data, extra)
	data[keyPlugin] = res.Plugin
	data[keyProcessMode] = res.ProcessMode
	data[keyPID] = res.PID
	data[keyFallback] = true
	data[keyFallbackReason] = res.FallbackReason
	data[keyRPCError] = res.RPCError
	if res.Host != nil {
		data["running"] = res.Host.Running
		data["stage"] = res.Host.Stage
		stderrTail := res.Host.StderrTail
		if stderrTail == nil {
			stderrTail = []string{}
		}
		data[keyStderrTail] = stderrTail
		if res.Host.Proc != nil {
			data[keyHost] = res.Host.Proc
		} else if res.Host.ProcError != "" {
			data[keyProcfsError] = res.Host.ProcError
		}
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map(data)}
}

func inspectPluginGoroutines(ctx *pluginserver.CommandContext, pluginName, mode string) (*plugin.Response, error) {
	srv, refusal := pluginInspectServer(ctx)
	if refusal != nil {
		return refusal, nil
	}
	res, opErr := inspectPluginFunc(srv, ctx.Context(), pluginName, rpc.RuntimeInspectInput{
		Kind: rpc.RuntimeInspectGoroutines,
	})
	if opErr != nil {
		return plugin.OperatorErrorResponse(opErr), nil
	}
	if res.Fallback {
		return pluginInspectFallbackResponse(res, map[string]any{keyMode: mode}), nil
	}

	data := buildGoroutinesMap(mode, res.Output.GoroutineStacks)
	data[keyPlugin] = res.Plugin
	data[keyProcessMode] = res.ProcessMode
	data[keyPID] = res.PID
	if res.Output.Truncated {
		data[triageKeyTruncated] = true
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map(data)}, nil
}

func inspectPluginProfile(
	ctx *pluginserver.CommandContext,
	pluginName, profileType string,
	duration time.Duration,
	wantSummary bool,
) (*plugin.Response, error) {
	srv, refusal := pluginInspectServer(ctx)
	if refusal != nil {
		return refusal, nil
	}
	input := rpc.RuntimeInspectInput{
		Kind:        rpc.RuntimeInspectProfile,
		ProfileType: profileType,
	}
	if profileType == profileTypeCPU {
		input.DurationMS = duration.Milliseconds()
	}

	res, opErr := inspectPluginFunc(srv, ctx.Context(), pluginName, input)
	if opErr != nil {
		return plugin.OperatorErrorResponse(opErr), nil
	}
	if res.Fallback {
		return pluginInspectFallbackResponse(res, map[string]any{keyType: profileType}), nil
	}

	if wantSummary {
		rawBytes, err := base64.StdEncoding.DecodeString(res.Output.ProfileData)
		if err != nil {
			return plugin.OperatorErrorResponse(opererr.Wrap(
				err,
				"cannot decode "+profileType+" profile from plugin "+strconv.Quote(pluginName),
				"the plugin returned malformed base64 profile data",
				"ze show system profile type "+profileType+" | plugin "+pluginName,
			)), nil //nolint:nilerr // operator error in Response
		}
		summary, err := summarizePprof(profileType, rawBytes)
		if err != nil {
			return plugin.OperatorErrorResponse(opererr.Wrap(
				err,
				"cannot summarize "+profileType+" profile from plugin "+strconv.Quote(pluginName),
				"the captured pprof data could not be decoded",
				"ze show system profile type "+profileType+" | plugin "+pluginName,
			)), nil //nolint:nilerr // operator error in Response
		}
		if profileType == profileTypeCPU {
			summary["duration"] = duration.String()
		} else {
			summary["profile-count"] = res.Output.ProfileCount
		}
		if res.Output.SamplingRate != nil {
			summary[keySamplingRate] = *res.Output.SamplingRate
		}
		summary[keyPlugin] = res.Plugin
		summary[keyProcessMode] = res.ProcessMode
		summary[keyPID] = res.PID
		if res.Output.Truncated {
			summary[triageKeyTruncated] = true
		}
		return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map(summary)}, nil
	}

	data := map[string]any{
		keyType:        profileType,
		keyFormat:      profileFormatPprofBase64,
		keyData:        res.Output.ProfileData,
		keyPlugin:      res.Plugin,
		keyProcessMode: res.ProcessMode,
		keyPID:         res.PID,
	}
	if profileType == profileTypeCPU {
		data["duration"] = duration.String()
	} else {
		data[keyCount] = res.Output.ProfileCount
	}
	if res.Output.SamplingRate != nil {
		data[keySamplingRate] = *res.Output.SamplingRate
	}
	if res.Output.Truncated {
		data[triageKeyTruncated] = true
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map(data)}, nil
}

func inspectPluginRuntimeMemory(ctx *pluginserver.CommandContext, pluginName string) (*plugin.Response, error) {
	srv, refusal := pluginInspectServer(ctx)
	if refusal != nil {
		return refusal, nil
	}
	res, opErr := inspectPluginFunc(srv, ctx.Context(), pluginName, rpc.RuntimeInspectInput{
		Kind: rpc.RuntimeInspectRuntimeStats,
	})
	if opErr != nil {
		return plugin.OperatorErrorResponse(opErr), nil
	}
	if res.Fallback {
		return pluginInspectFallbackResponse(res, nil), nil
	}

	st := res.Output.RuntimeStats
	data := map[string]any{
		"alloc":          st.Alloc,
		"total-alloc":    st.TotalAlloc,
		keySys:           st.Sys,
		keyHeapAlloc:     st.HeapAlloc,
		"heap-sys":       st.HeapSys,
		keyHeapInUse:     st.HeapInuse,
		"heap-objects":   st.HeapObjects,
		"stack-in-use":   st.StackInuse,
		keyNumGC:         st.NumGC,
		"gc-cpu-pct":     st.GCCPUPct,
		keyNumGoroutines: st.NumGoroutines,
		keyNumCPU:        st.NumCPU,
		keyMaxProcs:      st.MaxProcs,
		"go-version":     st.GoVersion,
		keyPlugin:        res.Plugin,
		keyProcessMode:   res.ProcessMode,
		keyPID:           res.PID,
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map(data)}, nil
}
