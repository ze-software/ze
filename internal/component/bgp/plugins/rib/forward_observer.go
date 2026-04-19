// Design: plan/design-rib-rs-fastpath.md -- observability subscriber for Change.Forward
// Related: rib.go -- SetLocRIB registers the observer
// Related: forward_handle.go -- ribForwardHandle is the producer side

package rib

import (
	"context"
	"log/slog"

	"codeberg.org/thomas-mangin/ze/internal/core/rib/locrib"
)

// observeForwardHandles registers a cheap OnChange subscriber on loc
// that emits a debug log line whenever a Change carries a non-nil
// Forward handle. This is the first real-code consumer of the producer
// wiring landed by design-rib-rs-fastpath: it does NOT AddRef or read
// Bytes (a nil-check is enough for presence observability).
//
// Intent: give operators one-line-per-best-change visibility that the
// locrib pipeline is receiving zero-copy-eligible BGP UPDATEs. Useful
// for verifying deployment-time wiring and for debugging why a future
// Forward-consuming plugin (RS/RR fast-path, sysrib mirroring, etc.)
// sees nil handles it was not expecting.
//
// Enable with `ze.log.bgp.rib=debug` or any level that includes debug
// for the bgp.rib subsystem. Enable with care at boot time: when
// active, every best-path change pays three `.String()` calls and a
// slog record allocation, all under the RIB write lock. Routine
// production logging levels (warn, info) skip the work entirely via
// the Enabled() pre-check.
//
// Returns the unsubscribe function returned by loc.OnChange. The caller
// MUST invoke it before dropping the loc reference or before rewiring
// to a different locrib; otherwise the subscription leaks. SetLocRIB
// handles this automatically.
func observeForwardHandles(loc *locrib.RIB) func() {
	return loc.OnChange(func(c locrib.Change) {
		if c.Forward == nil {
			return
		}
		lg := logger()
		if !lg.Enabled(context.Background(), slog.LevelDebug) {
			return
		}
		lg.Debug("forward-handle observed",
			"family", c.Family.String(),
			"prefix", c.Prefix.String(),
			"kind", c.Kind.String())
	})
}
