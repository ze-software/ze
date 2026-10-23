// Design: docs/architecture/update-building.md -- the FlowSpec redirect-to-IP next hop on the forward rails.
// Related: flowspec_redirect_forward_wire_test.go -- the redirect next hop a route keeps or resets when its action stands.
package reactor

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
)

// TestFlowSpecRedirectRefusedForward proves the general forward rail sends a
// FlowSpec route whose redirect action the RIB refused without its 0x0800
// community and with the zero next-hop length, and leaves a route the RIB did
// not refuse alone.
//
// draft-simpson-idr-flowspec-redirect-02 Section 3: "the BGP speaker SHOULD
// discard the redirect-to-ip extended community (and not propagate it further
// with the flow-spec route)".
//
// Method: the destination is EBGP under next hop unchanged, the mode that
// keeps the received redirect address, and the RIB's verdict is registered
// for the received UPDATE's own generation.
//
// MUTATION: drop the mpRedirectRefused branch of applyNextHopFamily (the
// refused route keeps its action and address).
func TestFlowSpecRedirectRefusedForward(t *testing.T) {
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}
	raw := mustHex(t, "050118c00002")
	received := netip.MustParseAddr("192.0.2.77").AsSlice()
	for _, tc := range []struct {
		name    string
		refused bool
	}{
		{"action-stands", false},
		{"action-refused", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newAIGPReplayFixture(t, nil)
			for _, peer := range []*Peer{f.source, f.destination} {
				negotiateForwardTestFamilies(peer, fam)
			}
			f.destination.settings.PeerAS = 65001
			f.destination.settings.NextHopMode = NextHopUnchanged
			f.destination.refreshForwardFacts()
			source := f.source.Settings().Address
			key := ribevents.ValidationRoute{Peer: source, Family: fam, NLRI: string(raw)}
			ribevents.RegisterFlowSpecLookup(func(got ribevents.ValidationRoute, _ uint64) bool { return got == key }, nil, nil, nil)
			t.Cleanup(func() { ribevents.RegisterFlowSpecLookup(nil, nil, nil, nil) })
			id := f.receive(t, flowRedirectPayload(t, fam, raw, received))
			ribevents.RegisterFlowSpecRedirectLookup(func(got ribevents.ValidationRoute, msgID uint64) bool {
				return tc.refused && got == key && msgID == id
			})
			t.Cleanup(func() { ribevents.RegisterFlowSpecRedirectLookup(nil) })
			f.forwardViaRIB(t, id)
			forwardSocketBarrier(t, f.r)
			bodies := aigpSocketBodies(t, f.conn)
			if len(bodies) != 1 {
				t.Fatalf("recipient UPDATE count=%d, want one rule", len(bodies))
			}
			want := received
			if tc.refused {
				want = nil
			}
			assertFlowRedirectWire(t, bodies[0], fam, raw, want, !tc.refused)
		})
	}
}

// TestFlowSpecRedirectRefusedRouteServer runs the check end to end on the
// route-server rail: the daemon's RIB judges the rule from EBGP client AS
// 65002, whose redirect target 192.0.2.77 resolves to 192.0.2.0/24 with
// AS_PATH 65002 65009. The neighbor is the client but the origin, the last
// AS, is not, so the client receives the rule without the action and with
// the zero next-hop length. TestFlowSpecRedirectRouteServerKeepsNextHop is
// the same path with origin 65002, where the action stands.
//
// draft-simpson-idr-flowspec-redirect-02 Section 3: "if the last AS in the
// AS_PATH or AS4_PATH attribute of the longest prefix match for X does not
// match the AS of the EBGP peer."
//
// MUTATION: drop the refused copy's flowSpecRedirectRefused from
// forwardUpdateValidated (the client receives the action and the received
// address).
func TestFlowSpecRedirectRefusedRouteServer(t *testing.T) {
	peers := flowForwardLiveRouter(t, "auto")
	peers[0].send(t, flowForwardFrame(mustHex(t, "000000184001010040020a02020000fdea0000fdf1400304c000020118c00002")))
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}
	raw := mustHex(t, "050118c00002")
	hop := netip.MustParseAddr("192.0.2.77").AsSlice()
	peers[0].send(t, flowForwardFrame(flowRedirectPayload(t, fam, raw, hop)))
	lowEventually(t, func() bool { return flowForwardReceived(peers[1], fam) != nil }, "authorized FlowSpec announcement")
	assertFlowRedirectWire(t, flowForwardReceived(peers[1], fam), fam, raw, nil, false)
}

// TestFlowSpecRedirectRefusedReplay proves a verdict that flips after the rule
// is in reaches the client through the validation replay, with or without the
// action as the new verdict says. The replay forwards the stored route under a
// fresh cache id, so a rail that asks the RIB by that id rather than by the
// received generation the RIB judged sends the refused action again.
//
// draft-simpson-idr-flowspec-redirect-02 Section 3: "the BGP speaker SHOULD
// discard the redirect-to-ip extended community (and not propagate it further
// with the flow-spec route) if the last AS in the AS_PATH or AS4_PATH
// attribute of the longest prefix match for X does not match the AS of the
// EBGP peer."
//
// Method: the daemon's own RIB and route server (flowForwardLiveRouter). The
// covering route for X first originates in the client's AS 65002, so the
// action stands; the client then replaces it with one originated by 65009,
// so the verdict flips to refused with no new FlowSpec UPDATE; then back.
//
// MUTATION: set validationSelection.msgID from the forward cache id in
// forwardValidationWire (the replayed route keeps its action).
func TestFlowSpecRedirectRefusedReplay(t *testing.T) {
	peers := flowForwardLiveRouter(t, "auto")
	ownOrigin := mustHex(t, "000000144001010040020602010000fdea400304c000020118c00002")
	otherOrigin := mustHex(t, "000000184001010040020a02020000fdea0000fdf1400304c000020118c00002")
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}
	raw := mustHex(t, "050118c00002")
	hop := netip.MustParseAddr("192.0.2.77").AsSlice()
	peers[0].send(t, flowForwardFrame(ownOrigin))
	peers[0].send(t, flowForwardFrame(flowRedirectPayload(t, fam, raw, hop)))
	lowEventually(t, func() bool { return len(flowForwardFrames(peers[1], fam)) == 1 }, "authorized FlowSpec redirect announcement")
	assertFlowRedirectWire(t, flowForwardFrames(peers[1], fam)[0], fam, raw, hop, true)

	peers[0].send(t, flowForwardFrame(otherOrigin))
	lowEventually(t, func() bool { return len(flowForwardFrames(peers[1], fam)) == 2 }, "replayed FlowSpec rule after the verdict flipped to refused")
	assertFlowRedirectWire(t, flowForwardFrames(peers[1], fam)[1], fam, raw, nil, false)

	peers[0].send(t, flowForwardFrame(ownOrigin))
	lowEventually(t, func() bool { return len(flowForwardFrames(peers[1], fam)) == 3 }, "replayed FlowSpec rule after the verdict flipped back")
	assertFlowRedirectWire(t, flowForwardFrames(peers[1], fam)[2], fam, raw, hop, true)
}

// flowForwardFrames returns every UPDATE body the peer received whose
// MP_REACH_NLRI carries fam, in arrival order.
func flowForwardFrames(peer *lowLivePeer, fam family.Family) [][]byte {
	peer.mu.Lock()
	defer peer.mu.Unlock()
	var bodies [][]byte
	for _, frame := range peer.frames {
		if frame[18] != 2 {
			continue
		}
		update, err := message.UnpackUpdate(frame[message.HeaderLen:])
		if err != nil {
			continue
		}
		_, _, mp, found := attribute.AttrFind(update.PathAttributes, attribute.AttrMPReachNLRI)
		if found && len(mp) > 5 && mp[0] == byte(fam.AFI>>8) && mp[1] == byte(fam.AFI) && mp[2] == byte(fam.SAFI) {
			bodies = append(bodies, bytes.Clone(frame[message.HeaderLen:]))
		}
	}
	return bodies
}
