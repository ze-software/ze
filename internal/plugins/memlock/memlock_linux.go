// Design: docs/architecture/plugin/plugin-system.md -- memlock plugin
// Overview: memlock.go -- why the executable is locked
// Related: register.go -- the registration this init() records against

//go:build linux

package memlock

import (
	lockexe "filippo.io/mlockexe"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/textbuf"
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
// only when an operator names it in a `plugin { }` block.
//
// The whole mapped size is charged against RLIMIT_MEMLOCK, including the pages
// that are never faulted, so a ze binary of 40 MiB or more needs a limit well
// above the 8 MiB systemd default. The outcome is RECORDED rather than logged,
// because init() runs before the daemon has a logger; `show module list` is
// what says it out loud.
//
// The failure is SOFT: the daemon serves every session correctly with an
// unlocked executable, and only pays for a page fault when the kernel has
// evicted one under memory pressure.
//
// This init() runs before the one in register.go, because Go initializes the
// files of a package in filename order. Neither write reads the other, so the
// order does not matter and no reader has to know it.
func init() {
	_, err := lockexe.OnFault()
	if err == nil {
		registry.RecordSetup(pluginName, registry.SetupSucceeded, "")
		return
	}

	var reason textbuf.Buffer
	reason.Str("the executable is not locked in memory, so its pages can be evicted under memory pressure: ").
		Err(err).
		Str("; raise RLIMIT_MEMLOCK above the size of the ze binary, which the ze.service unit does with LimitMEMLOCK=infinity")
	registry.RecordSetup(pluginName, registry.SetupFailedSoft, reason.String())
}
