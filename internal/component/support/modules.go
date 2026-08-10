// Design: docs/architecture/core-design.md — module registry for support bundle

package support

import (
	"maps"
	"sort"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// moduleCollector gathers data for a single support module.
// It returns a JSON-serializable value and any error encountered.
// Errors are non-fatal: the orchestrator records the error in the
// module result and continues with the next module.
type moduleCollector func(opts *collectOptions) (any, error)

// collectOptions carries flags that affect collection behavior.
type collectOptions struct {
	ConfigPath string
	Since      string
	Sensitive  bool
	SinceTime  time.Time
}

// moduleRegistry is the SINGLE SOURCE OF TRUTH for the set of
// modules available in `ze support`. Adding a new module means one
// entry here; all consumers (help text, --list-modules, validation,
// collection) DERIVE from this map.
var moduleRegistry = map[string]moduleCollector{
	"version":    collectVersion,
	"doctor":     collectDoctor,
	"host":       collectHost,
	"config":     collectConfig,
	"crashes":    collectCrashes,
	"disk":       collectDisk,
	"interfaces": collectInterfaces,
	"routes":     collectRoutes,
	"neighbors":  collectNeighbors,
	"env":        collectEnv,
	"sysctl":     collectSysctl,
	"platform":   collectPlatform,
	"runtime":    collectRuntime,
	"dmesg":      collectDmesg,
	"sockets":    collectSockets,
	"modules":    collectModules,
	"conntrack":  collectConntrack,
	"fds":        collectFDs,
	"dns":        collectDNS,
	"firewall":   collectFirewall,
}

// ModuleNames returns the sorted list of registered module names.
func ModuleNames() []string {
	names := make([]string, 0, len(moduleRegistry))
	for k := range moduleRegistry {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// ModuleList returns the sorted module names joined with ", ".
func ModuleList() string {
	return textbuf.Join(ModuleNames(), ", ")
}

// filterModules returns the subset of moduleRegistry matching the
// include/exclude filters. Empty include means all modules. Returns
// an error string if any requested name is unknown.
func filterModules(include, exclude []string) (map[string]moduleCollector, string) {
	if len(include) > 0 {
		result := make(map[string]moduleCollector, len(include))
		for _, name := range include {
			fn, ok := moduleRegistry[name]
			if !ok {
				return nil, "unknown module: " + name + " (valid: " + ModuleList() + ")"
			}
			result[name] = fn
		}
		return result, ""
	}

	result := make(map[string]moduleCollector, len(moduleRegistry))
	maps.Copy(result, moduleRegistry)

	for _, name := range exclude {
		if _, ok := moduleRegistry[name]; !ok {
			return nil, "unknown module: " + name + " (valid: " + ModuleList() + ")"
		}
		delete(result, name)
	}

	return result, ""
}
