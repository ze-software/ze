// Design: docs/architecture/core-design.md -- egress suppression on the forward rails
// RFC: rfc/short/rfc1997.md -- well-known communities MUST NOT leave their scope
// Related: forward_local_pref.go -- the sibling unconditional RFC egress prohibition
// Related: reactor_api_forward.go -- forwardUpdateCore, the general forward rail
// Related: forward_rs.go -- reactorForwardRS, the route-server forward rail
package reactor

import "github.com/ze-software/ze/internal/component/bgp/wireu"

// wellKnownAllowsEgress answers RFC 1997 for one destination of a fan-out and
// counts the suppression when the answer is no.
//
// It is the ONE site both forward rails ask, for the reason applyFactsLocalPref
// (forward_local_pref.go) is: the rails that re-derived an egress rule
// independently disagreed about it. wireu.WellKnown.AllowsEgressTo holds the
// RFC's three scopes and the confederation reasoning behind them; this wrapper
// adds only the counter, because the suppression is not configurable and is
// never logged per route (ai/rules/performance.md), so the counter is the one
// surface an operator can see it on.
//
// THE SET IS SCANNED ONCE PER UPDATE, NOT ONCE PER DESTINATION. Both callers
// hoist wireu.ScanWellKnown out of their destination loop, the same shape
// srcHasLocalPref already has, so a fan-out of a thousand peers walks the
// attribute headers once and each destination pays two branch tests.
//
// Safe on a nil receiver and a nil registry: the RFC answer never depends on
// whether metrics are configured.
func (r *Reactor) wellKnownAllowsEgress(w wireu.WellKnown, isIBGP bool) bool {
	if w.AllowsEgressTo(isIBGP) {
		return true
	}
	if r != nil && r.rmetrics != nil {
		r.rmetrics.wellKnownSuppressed.With(w.BlockingName(isIBGP)).Inc()
	}
	return false
}
