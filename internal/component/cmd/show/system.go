// Design: docs/guide/command-catalogue.md -- system/* operational commands
// Related: show.go -- sibling show handlers (uptime, warnings, errors, interface)
// Related: internal/component/host -- inventory library (DetectCPU/DetectPlatform)
//   used for the `hardware` enrichment below

package show

import (
	"encoding/json"
	"errors"
	"maps"
	"runtime"
	"time"

	"github.com/ze-software/ze/internal/component/host"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
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
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map(data)}, nil
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
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map(data)}, nil
}

// handleShowSystemSubsystemList returns available subsystems with their state.
func handleShowSystemSubsystemList(ctx *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	if ctx == nil || ctx.Server == nil {
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data:   plugin.Map{keySubsystems: []any{}, keyCount: 0},
		}, nil
	}
	pm := ctx.Server.ProcessManager()
	if pm == nil {
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data:   plugin.Map{keySubsystems: []any{}, keyCount: 0},
		}, nil
	}
	// The dispatcher's registry is what command-count counts. A per-Process list
	// was the source until 2026-08-18 and nothing had fed it since the YANG RPC
	// migration, so every operator read 0.
	var counts map[string]int
	if d := ctx.Server.Dispatcher(); d != nil {
		counts = d.Registry().CommandCountsByProcess()
	}
	procs := pm.AllProcesses()
	out := make([]map[string]any, 0, len(procs))
	for _, p := range procs {
		out = append(out, map[string]any{
			"name":          p.Name(),
			"stage":         p.Stage().String(),
			"running":       p.Running(),
			"command-count": counts[p.Name()],
		})
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{keySubsystems: out, keyCount: len(out)},
	}, nil
}

// handleShowSystemPlatform reports the runtime platform type and capabilities.
func handleShowSystemPlatform(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	p, err := host.DetectPlatform()
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, nil //nolint:nilerr // operational error in Response
	}
	b, jsonErr := json.Marshal(p)
	if jsonErr != nil {
		return nil, jsonErr
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.RawJSON(b)}, nil
}

// handleShowSystemDate reports the daemon's current wall-clock view in
// RFC3339, Unix seconds, and the configured timezone name.
func handleShowSystemDate(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	now := time.Now()
	zone, offset := now.Zone()
	data := map[string]any{
		"time":            now.Format(time.RFC3339),
		"unix":            now.Unix(),
		"unix-nano":       now.UnixNano(),
		"timezone":        zone,
		"utc-offset-secs": offset,
	}
	if ntp := registry.GetNTPSyncInfo(); ntp != nil {
		maps.Copy(data, ntp)
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map(data),
	}, nil
}
