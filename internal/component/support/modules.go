// Design: docs/architecture/core-design.md -- module registry for support bundle
// Related: register.go -- static and core runtime support module registrations
// Related: runtime_collectors.go -- core live runtime support bundle collectors

package support

import (
	"maps"

	"github.com/ze-software/ze/internal/core/supportreg"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// moduleCollector gathers data for a single support module.
// It returns a JSON-serializable value and any error encountered.
// Errors are non-fatal: the orchestrator records the error in the
// module result and continues with the next module.
type moduleCollector = supportreg.Collector

// collectOptions carries flags that affect collection behavior.
type collectOptions = supportreg.CollectOptions

// ModuleNames returns the sorted list of registered module names.
func ModuleNames() []string {
	return supportreg.Names()
}

// ModuleList returns the sorted module names joined with ", ".
func ModuleList() string {
	return textbuf.Join(ModuleNames(), ", ")
}

// filterModules returns the subset of registered modules matching the
// include/exclude filters. Empty include means all modules. Returns
// an error string if any requested name is unknown.
func filterModules(include, exclude []string) (map[string]moduleCollector, string) {
	reg := supportreg.Snapshot()
	if len(include) > 0 {
		result := make(map[string]moduleCollector, len(include))
		for _, name := range include {
			fn, ok := reg[name]
			if !ok {
				return nil, "unknown module: " + name + " (valid: " + ModuleList() + ")"
			}
			result[name] = fn
		}
		return result, ""
	}

	result := make(map[string]moduleCollector, len(reg))
	maps.Copy(result, reg)

	for _, name := range exclude {
		if _, ok := reg[name]; !ok {
			return nil, "unknown module: " + name + " (valid: " + ModuleList() + ")"
		}
		delete(result, name)
	}

	return result, ""
}
