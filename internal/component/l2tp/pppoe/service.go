// Design: docs/architecture/l2tp/bng-5-pppoe.md -- service locator for CLI access

package pppoe

import "sync/atomic"

// globalService holds the in-process PPPoE Subsystem. nil when the
// subsystem has not yet started (or has already stopped).
var globalService atomic.Pointer[Subsystem]

// PublishService stores s as the in-process PPPoE service handle.
// Called from Start; cleared from Stop.
func PublishService(s *Subsystem) {
	globalService.Store(s)
}

// LookupService returns the currently-published PPPoE subsystem, or
// nil when the subsystem has not yet started (or has already stopped).
func LookupService() *Subsystem {
	return globalService.Load()
}
