// Design: docs/architecture/plugin/plugin-system.md -- memlock plugin
// Overview: memlock.go -- why the executable is locked
// Related: register_lock.go -- the init() whose outcome setupOutcome words

//go:build linux

package memlock

import (
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// setupOutcome answers the outcome and the reason one locking call earns.
//
// It is a function rather than a branch inside init() so the soft-failure
// reason can be driven from a test. Which branch init() takes is decided by
// the host's RLIMIT_MEMLOCK, so a test that reads the recorded row asserts
// whichever branch that host happened to take, and on a host that can lock,
// the reason text below is never read at all.
func setupOutcome(err error) (registry.SetupOutcome, string) {
	if err == nil {
		return registry.SetupSucceeded, ""
	}

	var reason textbuf.Buffer
	reason.Str("the executable is not locked in memory, so its pages can be evicted under memory pressure: ").
		Err(err).
		Str("; raise RLIMIT_MEMLOCK above the size of the ze binary, which the ze.service unit does with LimitMEMLOCK=infinity")
	return registry.SetupFailedSoft, reason.String()
}
