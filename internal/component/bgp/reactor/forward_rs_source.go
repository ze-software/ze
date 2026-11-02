// Design: docs/architecture/bgp/structural-forwarding.md -- reactor-native RS forwarding
// Related: forward_rs.go -- reactorForwardRS, which reads the source classification
// Related: reactor_notify.go -- notifyMessageReceiver, which classifies at receipt

package reactor

import (
	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
)

// hasActiveFilter reports whether the per-peer chain must execute or refuse a
// reference. In-process ownership is proved only for import; a manually built
// export policy containing an active internal ref must not take a bypass.
func hasActiveFilter(policy filterapi.Policy, direction string) bool {
	if policy.Library() != nil {
		return true // the in-process chain is an active filter in both directions
	}
	for i, ref := range policy.Filters() {
		if policy.ActiveExternal(i) {
			return true
		}
		if ref.Inactive {
			continue
		}
		if direction != directionImport {
			return true
		}
	}
	return false
}

// sourceUsesCachedForward latches at receipt, even with the native fast path
// disabled. Enabling it later MUST NOT let a withdrawal overtake cached work.
func sourceUsesCachedForward(peer *Peer, update *wireu.WireUpdate) bool {
	if peer != nil && peer.forwardCached.Load() {
		return true
	}
	if ribevents.ValidationEnabled() || flowSpecUpdate(update) || len(payloadAIGP(update.Payload())) != 0 {
		if peer != nil {
			peer.forwardCached.Store(true)
		}
		return true
	}
	return false
}
