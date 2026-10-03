package filter_community_match

import (
	"testing"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// rfc8955Filter installs one community-match list, "POLICY", that accepts
// 65001:100 and rejects 65001:666, and runs one UPDATE through the filter
// entry point the engine calls.
func rfc8955Filter(t *testing.T, update string) sdk.FilterAction {
	t.Helper()
	old := listsByName.Load()
	t.Cleanup(func() { listsByName.Store(old) })
	lists := map[string]*communityList{"POLICY": {entries: []communityEntry{
		{community: "65001:666", ctype: communityStandard, action: actionReject},
		{community: "65001:100", ctype: communityStandard, action: actionAccept},
	}}}
	listsByName.Store(&lists)
	return handleFilterUpdate(&sdk.FilterUpdateInput{Filter: "POLICY", Peer: "192.0.2.1", Update: update}).Action
}

// RFC requirement: RFC8955-3-1 positive -- community matching applies to the
// Flow Specification NLRI type as it does to unicast: a FlowSpec route
// (ipv4/flow and ipv4/flow-vpn) carrying the permitted community 65001:100 is
// accepted by the community-match policy.
//
// VALIDATES: RFC 8955 Section 3 "community matching, must apply to the Flow specification defined NLRI-type".
// PREVENTS: the community policy being keyed on unicast NLRI text alone.
func TestRFC8955CommunityPolicyAcceptsAPermittedFlowSpecRoute(t *testing.T) {
	for _, nlri := range []string{"ipv4/flow add 10.1.0.0/24", "ipv4/flow-vpn add 10.1.0.0/24"} {
		if got := rfc8955Filter(t, "origin igp community 65001:100 nlri "+nlri); got != sdk.FilterAccept {
			t.Fatalf("%s with 65001:100: action = %v, want accept", nlri, got)
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
func TestRFC8955CommunityPolicyRejectsADeniedFlowSpecRoute(t *testing.T) {
	updates := []string{
		"origin igp community [65001:666 65001:100] nlri ipv4/flow add 10.1.0.0/24",
		"origin igp community 64496:1 nlri ipv4/flow add 10.1.0.0/24",
	}
	for _, update := range updates {
		if got := rfc8955Filter(t, update); got != sdk.FilterReject {
			t.Fatalf("%q: action = %v, want reject", update, got)
		}
	}
}
