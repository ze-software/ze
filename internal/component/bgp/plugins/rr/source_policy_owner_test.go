// Design: docs/architecture/api/process-protocol.md -- source-policy lifecycle

package rr

import "testing"

// TestRouteReflectorDeclaresSourcePolicyOwnership proves bgp-rr's Stage 1
// declaration names it a source-policy owner on its own, and leaves handler
// support to the SDK, which sets it at Run from the OnSourcePolicy handler
// serveRouteReflector registers (proved over both transports by
// TestRFC8212RouteReflectorSourcePolicyTransport).
// Goal: the engine never treats bgp-rr as an observer and leaves the
// forwarded routes in place, and support is declared only by registering the
// handler that performs it. Method: read the registration Run sends.
func TestRouteReflectorDeclaresSourcePolicyOwnership(t *testing.T) {
	reg := routeReflectorRegistration()
	if !reg.SourcePolicyOwner {
		t.Fatal("bgp-rr holds per-source routes and must declare source-policy-owner")
	}
	if reg.SourcePolicy {
		t.Fatal("bgp-rr's compiled declaration must not claim handler support: the SDK sets it from OnSourcePolicy")
	}
}
