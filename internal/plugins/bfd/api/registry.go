// Design: rfc/short/rfc5882.md -- BFD client lookup contract
// Related: service.go -- Service / SessionHandle interfaces
// Related: events.go -- SessionRequest / Key / StateChange
//
// In-process Service lookup. Clients (BGP, OSPF, static-route monitors)
// that run in the same ze process as the BFD engine reach the live
// Service implementation via SetService/GetService instead of importing
// internal/plugins/bfd/engine directly. This keeps the import graph
// clean: BGP only pulls in internal/plugins/bfd/api (a leaf package with
// no runtime dependencies), and the bfd plugin publishes its concrete
// Service in OnStarted.
//
// Lifecycle: the bfd plugin SetServices its engine handle when the
// OnStarted callback fires, and clears it via SetService(nil) in the
// deferred shutdown path. A client that calls GetService before the
// bfd plugin is ready receives nil -- callers MUST check the return
// value and skip BFD wiring gracefully (BGP still runs without BFD).
//
// Thread safety: SetService/GetService use an atomic.Pointer so a
// client reading the pointer races cleanly with the one writer (the
// bfd plugin's SDK callback goroutine). No mutex, no lock ordering
// constraints to document.
//
// External (forked) plugin support is explicitly out of scope: an
// external BGP plugin would need a DispatchCommand-based protocol
// shim to reach the BFD engine across the pipe boundary. That is
// tracked as spec-bfd-3b-external-bgp-bfd.
package api

import "sync/atomic"

// globalService holds the in-process BFD Service. nil when the bfd
// plugin has not yet published its engine or has shut down.
//
// We use an atomic.Pointer to an interface value rather than storing
// the interface directly so readers observe a consistent pointer (an
// interface value is two words and cannot be written atomically).
var globalService atomic.Pointer[Service]

// SetService publishes svc as the global BFD service. Called by the
// bfd plugin's OnStarted callback once its engine is running. Pass
// nil in the shutdown path to clear the publication so late clients
// observe "no BFD available" instead of a handle whose methods race
// the teardown.
//
// Safe for concurrent use.
func SetService(svc Service) {
	if svc == nil {
		globalService.Store(nil)
		return
	}
	globalService.Store(&svc)
}

// GetService returns the currently-published BFD Service, or nil if
// the bfd plugin has not yet run SetService (or has already cleared
// it). Callers MUST handle the nil return: BGP, OSPF, and other
// clients run without BFD rather than fail their own session
// lifecycle.
//
// Safe for concurrent use.
func GetService() Service {
	p := globalService.Load()
	if p == nil {
		return nil
	}
	return *p
}
