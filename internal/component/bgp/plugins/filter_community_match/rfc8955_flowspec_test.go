package filter_community_match

import "testing"

// rfc8955List is one community-match list, "POLICY", that rejects 65001:666
// and accepts 65001:100, in that order.
func rfc8955List() map[string]any {
	return map[string]any{"entry": []any{
		map[string]any{"community": "65001:666", "action": "reject"},
		map[string]any{"community": "65001:100", "action": "accept"},
	}}
}

// RFC requirement: RFC8955-3-1 positive -- community matching applies to the
// Flow Specification NLRI type as it does to unicast: a FlowSpec route
// (ipv4/flow and ipv4/flow-vpn) carrying the permitted community 65001:100 is
// accepted by the community-match policy.
//
// VALIDATES: RFC 8955 Section 3 "community matching, must apply to the Flow specification defined NLRI-type".
// PREVENTS: the community policy being keyed on unicast NLRI text alone.
// Method: each UPDATE carries the FlowSpec rule in MP_REACH_NLRI and no IPv4
// NLRI field (flowSpecUpdate), so nothing but the FlowSpec route is judged.
func TestRFC8955CommunityPolicyAcceptsAPermittedFlowSpecRoute(t *testing.T) {
	for _, safi := range []uint8{133, 134} {
		attrs := append(flowSpecReach(safi), standardCommunities(65001<<16|100)...)
		if got := judgeWire(t, rfc8955List(), flowSpecUpdate(t, attrs)); got != actionAccept {
			t.Fatalf("SAFI %d with 65001:100: action = %v, want accept", safi, got)
		}
	}
}

// RFC requirement: RFC8955-3-1 negative -- a FlowSpec route cannot bypass the
// community policy: an ipv4/flow route carrying the denied community 65001:666
// (beside the permitted 65001:100, which the first match outranks) and an
// ipv4/flow route carrying no listed community are each rejected.
//
// VALIDATES: the community-match deny and implicit deny reach FlowSpec routes.
// PREVENTS: a FlowSpec rule the operator's community policy refuses being installed.
// Method: FlowSpec-only UPDATEs (flowSpecUpdate, no IPv4 NLRI field), so a
// community check skipped when no unicast NLRI is present goes red here.
func TestRFC8955CommunityPolicyRejectsADeniedFlowSpecRoute(t *testing.T) {
	for name, communities := range map[string][]byte{
		"65001:666 beside 65001:100": standardCommunities(65001<<16|666, 65001<<16|100),
		"64496:1 only":               standardCommunities(64496<<16 | 1),
	} {
		attrs := append(flowSpecReach(133), communities...)
		if got := judgeWire(t, rfc8955List(), flowSpecUpdate(t, attrs)); got != actionReject {
			t.Fatalf("%s: action = %v, want reject", name, got)
		}
	}
}
