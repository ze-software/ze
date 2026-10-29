// Design: docs/architecture/api/process-protocol.md -- source-policy lifecycle

package rs

import "testing"

// TestRouteServerDeclaresSourcePolicyOwnership proves bgp-rs's Stage 1
// declaration names it a source-policy owner without lifecycle support.
// Goal: the engine's preflight refuses an import-policy removal for a peer
// bgp-rs forwards, rather than treating it as an observer and leaving the
// forwarded routes in place. Method: read the registration Run sends.
func TestRouteServerDeclaresSourcePolicyOwnership(t *testing.T) {
	reg := routeServerRegistration()
	if !reg.SourcePolicyOwner {
		t.Fatal("bgp-rs holds per-source routes and must declare source-policy-owner")
	}
	if reg.SourcePolicy {
		t.Fatal("bgp-rs registers no source-policy handler and must not declare support")
	}
}
