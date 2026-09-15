// Design: docs/architecture/diagnostics/debug-filtering.md -- debug CLI registration
// Related: debug.go -- verb-first set/delete/show/clear handlers

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
		ShortHelp: "Enable debug output for one subsystem.",
		Description: "A level, a flag or a scope can be set in the same command, and each one narrows " +
			"what the subsystem writes.",
	})
	registry.MustRegisterLocalMeta("delete debug module", runDeleteModule, registry.Meta{
		ShortHelp: "Disable debug for a subsystem, or remove one of its flags/scopes.",
		Description: "The module name alone removes the whole module. Naming a flag or a scope removes " +
			"that one and leaves the module enabled. Deleting a module the profile does not hold " +
			"succeeds and changes nothing, so a repeated command is safe.",
	})
	registry.MustRegisterLocalMeta("set debug timeout", runSetTimeout, registry.Meta{
		ShortHelp: "Set how long debug output stays enabled.",
		Description: "The duration is written as 30m, 1h or 90s, seconds are rounded up to minutes, " +
			"and the longest accepted value is 24h. Zero disables the timer.",
	})
	registry.MustRegisterLocalMeta("set debug profile name", runSaveProfile, registry.Meta{
		ShortHelp: "Save the current debug state as a named profile.",
		Description: "It copies what the daemon is writing NOW into a named slot. The default slot is " +
			"left as it is, so saving a profile does not change what the running daemon logs.",
	})
	registry.MustRegisterLocalMeta("set debug active name", runRestoreProfile, registry.Meta{
		ShortHelp: "Load a named debug profile and apply it to the running daemon.",
		Description: "The named profile becomes the live state and the default slot is NOT overwritten, " +
			"so a restart returns the daemon to the default profile rather than to this one.",
	})
	registry.MustRegisterLocalMeta("delete debug profile name", runDeleteProfileName, registry.Meta{
		ShortHelp: "Delete a named debug profile.",
		Description: "It removes the stored slot and nothing else. The running daemon keeps whatever it " +
			"is writing, so deleting the profile that is live does not turn its output off.",
	})
	registry.MustRegisterLocalMeta("clear debug", func(_ []string) int { return cmdClear() }, registry.Meta{
		ShortHelp: "Clear the default debug profile.",
		Description: "It writes an empty profile into the default slot AND applies it, so the stored " +
			"default and the running daemon both stop. A named profile is not touched.",
	})
	registry.MustRegisterLocalMeta("show debug profile", runShowProfile, registry.Meta{
		ShortHelp: "Show stored debug profiles, one by name, or one filtered to a module subtree.",
		Description: "With no argument it lists the profile names. `name <name>` prints that profile as a " +
			"table of module, level, flags and scopes. Adding `module <prefix>` keeps the rows under " +
			"one subsystem subtree. Any other trailing word is refused rather than ignored.",
	})
}
