// Design: docs/architecture/plugin/plugin-system.md -- memlock plugin
// Overview: memlock.go -- why the executable is locked

//go:build linux

package memlock

import lockexe "filippo.io/mlockexe"

// lockedOctets and lockErr record what the init() below achieved, for the
// doctor check in register.go to report. They are written once, by that
// init(), before main() and before any goroutine exists, and never again.
// Safe for concurrent use.
var (
	lockedOctets int64
	lockErr      error
)

// init locks the mappings of the running executable with mlock2(2) and
// MLOCK_ONFAULT. Resident pages are locked now, and the rest as they are
// faulted in, so startup pays for no page this process never executes.
//
// It runs here, at package init, and not in the plugin's engine run. A page
// evicted before the lock is a page the lock must fault back in, and the
// engine run is later still: this plugin declares no config root, no family
// and no event type, so nothing auto-loads it and the engine run is reached
// only when an operator names it in a `plugin { }` block.
//
// The whole mapped size is charged against RLIMIT_MEMLOCK, including the pages
// that are never faulted, so a ze binary of 40 MiB or more needs a limit well
// above the 8 MiB systemd default. The outcome is recorded rather than logged
// here, because init() runs before the daemon has a logger. The doctor check
// is what says it out loud.
func init() {
	lockedOctets, lockErr = lockexe.OnFault()
}
