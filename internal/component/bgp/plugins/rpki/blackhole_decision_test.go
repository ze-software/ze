// VALIDATES: buildDecisions applies the RFC 7999 Section 3.3 exemption, and
// applies it only when all three of its conditions hold.
// PREVENTS: an operator running origin-invalid-action reject getting a blackhole
// honoring switch that never fires, and the exemption widening into a general
// escape from RFC 6811 origin validation.

package rpki

import "testing"

// blackholeDecisionPlugin builds a plugin whose global policy rejects Invalid
// routes, with one VRP covering 192.0.2.0/24 at maxLength 24 for AS 65001.
func blackholeDecisionPlugin(t *testing.T, exempt bool) *rPKIPlugin {
	t.Helper()
	rp := &rPKIPlugin{
		cache:         newROACache(),
		aspaCache:     newASPACache(),
		aspaTracker:   newASPATracker(),
		originTracker: newOriginTracker(),
	}
	rp.cache.Add(makeVRP("192.0.2.0/24", 24, 65001))
	rp.originInvalidAction.Store(uint32(ASPAPolicyReject))
	rp.originNotFoundAction.Store(uint32(ASPAPolicyAccept))
	rp.aspaInvalidAction.Store(uint32(ASPAPolicyAccept))
	rp.aspaUnknownAction.Store(uint32(ASPAPolicyAccept))

	peers := map[string]peerActionSet{
		"198.51.100.1": {
			OriginInvalid:   resolvedAction{Action: ASPAPolicyReject, Source: sourcePeer},
			OriginNotFound:  resolvedAction{Action: ASPAPolicyAccept, Source: sourcePeer},
			ASPAInvalid:     resolvedAction{Action: ASPAPolicyAccept, Source: sourcePeer},
			ASPAUnknown:     resolvedAction{Action: ASPAPolicyAccept, Source: sourcePeer},
			BlackholeExempt: exempt,
		},
	}
	rp.perPeerActions.Store(&peers)
	return rp
}

func blackholeRequest(prefix string, originAS uint32, blackhole bool) validationRequest {
	return validationRequest{
		peerAddr:  "198.51.100.1",
		family:    "ipv4/unicast",
		prefix:    prefix,
		state:     ValidationInvalid,
		aspaState: aspaStateNone,
		originAS:  originAS,
		blackhole: blackhole,
	}
}

// AC-5. The exemption is on, the route carries BLACKHOLE, and the only fault is
// that a /32 is longer than the covering VRP's maxLength 24. RFC 7999 Section
// 3.3 says origin validation must not block this announcement.
func TestBlackholeSurvivesLengthOnlyInvalid(t *testing.T) {
	rp := blackholeDecisionPlugin(t, true)

	decisions := rp.buildDecisions([]validationRequest{
		blackholeRequest("192.0.2.1/32", 65001, true),
	})

	if len(decisions) != 1 {
		t.Fatalf("got %d decisions, want 1", len(decisions))
	}
	if !decisions[0].Accept {
		t.Error("a legitimate BLACKHOLE announcement was rejected by origin validation")
	}
}

// AC-6. The same session, the same exemption, the same community, and an origin
// AS no covering VRP names. This is the hijack shape, and it stays rejected.
func TestBlackholeDoesNotSurviveAWrongOrigin(t *testing.T) {
	rp := blackholeDecisionPlugin(t, true)

	decisions := rp.buildDecisions([]validationRequest{
		blackholeRequest("192.0.2.1/32", 65999, true),
	})

	if decisions[0].Accept {
		t.Error("a BLACKHOLE announcement from an unauthorized origin was accepted: the exemption is a hijack path")
	}
}

// The community is required. Without it the exemption has nothing to protect,
// and an Invalid more-specific is an ordinary Invalid route.
func TestBlackholeExemptionNeedsTheCommunity(t *testing.T) {
	rp := blackholeDecisionPlugin(t, true)

	decisions := rp.buildDecisions([]validationRequest{
		blackholeRequest("192.0.2.1/32", 65001, false),
	})

	if decisions[0].Accept {
		t.Error("an untagged Invalid route was accepted under the blackhole exemption")
	}
}

// The operator must have asked. RFC 7999 Section 3.3 binds the agreement to one
// session, and the leaf defaults to false, so a session that said nothing keeps
// rejecting exactly as it does today.
func TestBlackholeExemptionNeedsTheOperatorToAsk(t *testing.T) {
	rp := blackholeDecisionPlugin(t, false)

	decisions := rp.buildDecisions([]validationRequest{
		blackholeRequest("192.0.2.1/32", 65001, true),
	})

	if decisions[0].Accept {
		t.Error("the exemption fired on a session that never asked for it")
	}
}

// A peer with no per-peer entry at all uses the global actions and no
// exemption. This is every peer in a deployment that does not use the feature.
func TestBlackholeExemptionAbsentForAnUnlistedPeer(t *testing.T) {
	rp := blackholeDecisionPlugin(t, true)

	req := blackholeRequest("192.0.2.1/32", 65001, true)
	req.peerAddr = "203.0.113.9"
	decisions := rp.buildDecisions([]validationRequest{req})

	if decisions[0].Accept {
		t.Error("the exemption reached a peer with no per-peer policy")
	}
}

// The exemption must not touch a state that is not Invalid. A NotFound route
// under an accept policy was already accepted, and its ValState must survive.
func TestBlackholeExemptionLeavesOtherStatesAlone(t *testing.T) {
	rp := blackholeDecisionPlugin(t, true)

	req := blackholeRequest("203.0.113.1/32", 65001, true)
	req.state = ValidationNotFound
	decisions := rp.buildDecisions([]validationRequest{req})

	if !decisions[0].Accept {
		t.Fatal("a NotFound route under an accept policy was rejected")
	}
	if decisions[0].ValState != ValidationNotFound {
		t.Errorf("ValState = %d, want NotFound: the exemption rewrote an unrelated state", decisions[0].ValState)
	}
}

// An accepted route keeps its real validation state, Invalid included. The
// exemption changes whether the route is kept, never what it is marked as: an
// operator looking at the Adj-RIB-In must still see the RFC 6811 verdict.
func TestBlackholeExemptionPreservesTheInvalidState(t *testing.T) {
	rp := blackholeDecisionPlugin(t, true)

	decisions := rp.buildDecisions([]validationRequest{
		blackholeRequest("192.0.2.1/32", 65001, true),
	})

	if !decisions[0].Accept {
		t.Fatal("the exempted route was rejected")
	}
	if decisions[0].ValState != ValidationInvalid {
		t.Errorf("ValState = %d, want Invalid: the route's RFC 6811 verdict was hidden", decisions[0].ValState)
	}
}
