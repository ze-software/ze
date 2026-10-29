// Design: docs/architecture/api/process-protocol.md -- source-policy lifecycle

package rr

import "testing"

// TestRouteReflectorDeclaresSourcePolicyOwnership proves bgp-rr's Stage 1
// declaration names it a source-policy owner without lifecycle support.
// Goal: the engine's preflight refuses an import-policy removal for a peer
// bgp-rr forwards, rather than treating it as an observer and leaving the
// forwarded routes in place. Method: read the registration Run sends.
func TestRouteReflectorDeclaresSourcePolicyOwnership(t *testing.T) {
	reg := routeReflectorRegistration()
	if !reg.SourcePolicyOwner {
		t.Fatal("bgp-rr holds per-source routes and must declare source-policy-owner")
	}
	if reg.SourcePolicy {
		t.Fatal("bgp-rr registers no source-policy handler and must not declare support")
	}
}
