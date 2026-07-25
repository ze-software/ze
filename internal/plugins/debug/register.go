// Design: plan/learned/891-granular-debug.md -- debug CLI registration
// Related: debug.go -- verb-first set/delete/show/clear handlers

// codegen:skip -- CLI commands wired via the command registry, not a runtime plugin.

package debug

import (
	"github.com/ze-software/ze/internal/component/command/registry"
)

func init() {
	// Offline debug management: verb-first (set/delete/show/clear), editing the
	// stored profile in debug.zefs which the daemon applies on load. Grammar
	// follows VyOS syslog-level configuration (docs.vyos.io). Registered as
	// local shortcuts under existing verb roots; cmdutil.RunCommand checks the
	// local registry before the daemon, so they run in-process without shadowing
	// the daemon `show debug` command (live runtime state, in yang/).
	registry.MustRegisterLocalMeta("set debug module", runSetModule, registry.Meta{
		Description: "Enable debug for a subsystem; optionally set level/flag/scope. E.g. 'set debug module bgp.reactor level debug'.",
	})
	registry.MustRegisterLocalMeta("delete debug module", runDeleteModule, registry.Meta{
		Description: "Disable debug for a subsystem, or remove one of its flags/scopes.",
	})
	registry.MustRegisterLocalMeta("set debug timeout", runSetTimeout, registry.Meta{
		Description: "Set the debug auto-disable timer (e.g. 30m, 1h, 90s; 0 disables).",
	})
	registry.MustRegisterLocalMeta("set debug profile name", runSaveProfile, registry.Meta{
		Description: "Save the current debug state as a named profile.",
	})
	registry.MustRegisterLocalMeta("set debug active name", runRestoreProfile, registry.Meta{
		Description: "Load a named debug profile and apply it to the running daemon.",
	})
	registry.MustRegisterLocalMeta("delete debug profile name", runDeleteProfileName, registry.Meta{
		Description: "Delete a named debug profile.",
	})
	registry.MustRegisterLocalMeta("clear debug", func(_ []string) int { return cmdClear() }, registry.Meta{
		Description: "Clear the default debug profile.",
	})
	registry.MustRegisterLocalMeta("show debug profile", runShowProfile, registry.Meta{
		Description: "Show stored debug profiles (list, 'name <name>' for one, add 'module <prefix>' to filter).",
	})
}
