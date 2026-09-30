// Design: docs/architecture/ospf/ospf-ext-1-opaque-framework.md -- RFC 5250 Type-11 originator reachability.
// Related: ext_prefix.go -- extPrefixUsable, the read-time RFC 5250 sec 5 gate.
// Related: ext_render.go -- extOpaqueDecode, the operator view of the resolved prefixes.
//
// VALIDATES: RFC 5250 Section 5, "It also MUST discontinue using all Opaque LSAs injected
// into the network by the same originator whenever it is detected that the originator is
// unreachable", for the Extended Prefix receiver: LSAs delivered while their originator was
// reachable are shown usable, and the same entries, with no new LSA arriving, are shown
// unusable once the routing table no longer reaches the originator.
// PREVENTS: a usability flag cached when the LSA arrived that outlives its originator.
package ospf

import (
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/transport"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// rfc5250ResolvedUsable returns the usability the AS-scope operator view reports for each
// prefix row of adv, keyed by the row's prefix text.
func rfc5250ResolvedUsable(eng *engine, adv types.RouterID) map[string]bool {
	out := map[string]bool{}
	for _, row := range eng.extOpaqueDecode(OpaqueScopeAS).ResolvedPrefixes {
		if row.AdvertisingRouter == adv.String() {
			out[row.Prefix] = row.Usable
		}
	}
	return out
}

// TestRFC5250DiscontinueOpaqueFromUnreachableOriginator delivers two Type-11 Extended Prefix
// LSAs (Opaque IDs 1 and 2) from one originator while it is reachable, then makes the
// originator unreachable through the engine's reachability seam and reads the view again.
func TestRFC5250DiscontinueOpaqueFromUnreachableOriginator(t *testing.T) {
	eng := newEngine(transport.New(&fakeBackend{}))
	defer eng.shutdown()
	adv := types.RouterID{2, 2, 2, 2}
	reachable := true
	eng.opaqueReachableFn = func(id types.RouterID) bool { return reachable && id == adv }

	for opaqueID, addr := range map[uint32][4]byte{1: {198, 51, 100, 0}, 2: {203, 0, 113, 0}} {
		eng.extPrefixOnReceive(opaqueReceived{
			OpaqueType: packet.ExtPrefixOpaqueType, OpaqueID: opaqueID, Scope: OpaqueScopeAS,
			AdvertisingRouter: adv, Reachable: true,
			Body: extPrefixBody(packet.ExtRouteTypeASExternal, 24, 0, addr),
		})
	}

	// RFC requirement: RFC5250-5-2 positive -- while the originator is reachable, both Type-11
	// Opaque LSAs it injected are in use: the AS-scope view shows 198.51.100.0/24 and
	// 203.0.113.0/24 from 2.2.2.2 as usable.
	before := rfc5250ResolvedUsable(eng, adv)
	if len(before) != 2 {
		t.Fatalf("resolved rows for %s = %v, want the two delivered prefixes", adv, before)
	}
	for prefix, usable := range before {
		if !usable {
			t.Fatalf("%s from reachable %s shown unusable", prefix, adv)
		}
	}

	// RFC requirement: RFC5250-5-2 negative -- once the originator is detected unreachable,
	// every Opaque LSA it injected stops being used, with no new LSA arriving: the same two
	// rows are shown unusable.
	reachable = false
	after := rfc5250ResolvedUsable(eng, adv)
	if len(after) != 2 {
		t.Fatalf("resolved rows for %s after it became unreachable = %v, want the same two", adv, after)
	}
	for prefix, usable := range after {
		if usable {
			t.Fatalf("%s is still shown usable after its originator %s became unreachable", prefix, adv)
		}
	}
}
