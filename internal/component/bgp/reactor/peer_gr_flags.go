// Design: docs/guide/graceful-restart.md -- what Ze advertises in the GR capability
// RFC: rfc/short/rfc4724.md
// Related: peer.go -- getPluginCapabilities, the one caller
// Related: internal/component/bgp/grmarker -- the two bit edits this chooses between

package reactor

import (
	"github.com/ze-software/ze/internal/component/bgp/grmarker"
	"github.com/ze-software/ze/internal/component/plugin"
)

// restartFlagsFor applies the two Restarting Speaker bits of the RFC 4724
// Graceful Restart capability to the payload a plugin built.
//
// Neither bit can be built with the payload. parseGRCapValue
// (internal/component/bgp/plugins/gr/gr_capability.go) runs when the
// configuration is loaded, and both bits report on a restart that the daemon
// learns about afterwards, from the marker and from the forwarding plane. So
// the payload carries the configured facts and this carries the runtime ones.
//
// inRestartWindow gates BOTH, because each speaks about "the previous BGP
// restart" and a cold start had none:
//
//   - RFC 4724 Section 3 on the Restart State bit: "When set (value 1), this
//     bit indicates that the BGP speaker has restarted".
//   - RFC 4724 Section 4.1 on the Forwarding State bit: it "can be set only if
//     the forwarding state has indeed been preserved for that address family
//     during the restart".
//
// forwardingPreserved is the second bit's own condition, answered by the
// plugin that owns the forwarding plane (registry.ForwardingStatePreserved).
// It is read only inside the window, so a cold start cannot reach it.
func restartFlagsFor(caps []plugin.InjectedCapability, inRestartWindow, forwardingPreserved bool) []plugin.InjectedCapability {
	if !inRestartWindow {
		return caps
	}
	caps = grmarker.SetRBit(caps)
	if forwardingPreserved {
		caps = grmarker.SetFBit(caps)
	}
	return caps
}
