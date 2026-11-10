// Design: docs/architecture/plugin/plugin-system.md -- memlock plugin
// Overview: memlock.go -- why the executable is locked
// Related: register.go -- the registration this init() records against
// Related: memlock_linux.go -- setupOutcome, the outcome this init() records

//go:build linux

package memlock

import (
	lockexe "filippo.io/mlockexe"

	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// init locks the mappings of the running executable with mlock2(2) and
// MLOCK_ONFAULT, then records the outcome in the plugin registry. Resident
// pages are locked now, and the rest as they are faulted in, so startup pays
// for no page this process never executes.
//
// It runs here, at package init, and not in the plugin's engine run. A page
// evicted before the lock is a page the lock must fault back in, and the
// engine run is later still: this plugin declares no config root, no family
// and no event type, so nothing auto-loads it and the engine run is reached
// only when an operator names it in a `plugin { }` block. It sits in a register
// file because recording the outcome is a write to the plugin registry, and
// every such write does (`./le plugin registration-sites check`).
//
// The whole mapped size is charged against RLIMIT_MEMLOCK, including the pages
// that are never faulted, so a ze binary of 40 MiB or more needs a limit well
// above the 8 MiB systemd default. The outcome is RECORDED rather than logged,
// because init() runs before the daemon has a logger; the `outcome` column of
// `show plugin list` is what says it out loud.
//
// The failure is SOFT: the daemon serves every session correctly with an
// unlocked executable, and only pays for a page fault when the kernel has
// evicted one under memory pressure.
//
// This init() runs after the one in register.go, because Go initializes the
// files of a package in filename order. Neither write reads the other, so the
// order does not matter and no reader has to know it.
func init() {
	_, err := lockexe.OnFault()
	outcome, reason := setupOutcome(err)
	registry.RecordSetup(pluginName, outcome, reason)
}
