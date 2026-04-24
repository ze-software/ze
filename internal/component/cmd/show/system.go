// Design: docs/guide/command-catalogue.md -- system/* operational commands
// Related: show.go -- sibling show handlers (uptime, warnings, errors, interface)
// Related: host.go -- `show host *` commands sharing the inventory library
//   used for the `hardware` enrichment below

package show

import (
	"errors"
	"runtime"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/host"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

// handleShowSystemMemory reports the current Go runtime memory statistics
// for the daemon process. Output is a flat JSON map with kebab-case keys
// matching MemStats field intent (alloc, total-alloc, sys, heap-in-use,
// heap-objects, num-gc). The `hardware` nested object surfaces the
// physical memory sizes and ECC counters from host inventory (Linux only;
// omitted entirely on platforms where inventory returns ErrUnsupported).
func handleShowSystemMemory(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	data := map[string]any{
		"alloc":        m.Alloc,
		"total-alloc":  m.TotalAlloc,
		"sys":          m.Sys,
		"heap-alloc":   m.HeapAlloc,
		"heap-sys":     m.HeapSys,
		"heap-in-use":  m.HeapInuse,
		"heap-objects": m.HeapObjects,
		"stack-in-use": m.StackInuse,
		"num-gc":       m.NumGC,
		"gc-cpu-pct":   m.GCCPUFraction * 100,
	}
	if hw, err := host.DetectMemory(); err == nil && hw != nil {
		data["hardware"] = hw
	} else if err != nil && !errors.Is(err, host.ErrUnsupported) {
		data["hardware-error"] = err.Error()
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: data}, nil
}

// handleShowSystemCPU reports goroutine count, logical CPU count, and
// GOMAXPROCS for the daemon process. The `hardware` nested object
// surfaces the physical CPU inventory (model, cores, hybrid layout,
// frequencies) on Linux; omitted on platforms where inventory returns
// ErrUnsupported so operators still get the runtime fields.
func handleShowSystemCPU(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	data := map[string]any{
		"num-cpu":        runtime.NumCPU(),
		"num-goroutines": runtime.NumGoroutine(),
		"max-procs":      runtime.GOMAXPROCS(0),
		"go-version":     runtime.Version(),
	}
	if hw, err := host.DetectCPU(); err == nil && hw != nil {
		data["hardware"] = hw
	} else if err != nil && !errors.Is(err, host.ErrUnsupported) {
		data["hardware-error"] = err.Error()
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: data}, nil
}

// handleShowSystemSubsystemList returns available subsystems with their state.
func handleShowSystemSubsystemList(ctx *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	if ctx == nil || ctx.Server == nil {
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data:   map[string]any{"subsystems": []any{}, "count": 0},
		}, nil
	}
	pm := ctx.Server.ProcessManager()
	if pm == nil {
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data:   map[string]any{"subsystems": []any{}, "count": 0},
		}, nil
	}
	procs := pm.AllProcesses()
	out := make([]map[string]any, 0, len(procs))
	for _, p := range procs {
		out = append(out, map[string]any{
			"name":          p.Name(),
			"stage":         p.Stage().String(),
			"running":       p.Running(),
			"command-count": len(p.RegisteredCommands()),
		})
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   map[string]any{"subsystems": out, "count": len(out)},
	}, nil
}

// handleShowSystemDate reports the daemon's current wall-clock view in
// RFC3339, Unix seconds, and the configured timezone name.
func handleShowSystemDate(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	now := time.Now()
	zone, offset := now.Zone()
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"time":            now.Format(time.RFC3339),
			"unix":            now.Unix(),
			"unix-nano":       now.UnixNano(),
			"timezone":        zone,
			"utc-offset-secs": offset,
		},
	}, nil
}
