// Design: docs/architecture/update-building.md -- the FlowSpec redirect-to-IP next hop on the forward rails.
// Related: flowspec_forward_wire_test.go -- the zero-length next hop every other FlowSpec route leaves with.
package reactor

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
)

// flowRedirectAction is the redirect/mirror-to-IP-next-hop extended community
// (type 0x0800, C bit clear): the route's action target is its next hop.
const flowRedirectAction = "0800000000000000"

// TestFlowSpecRedirectForwardNextHop proves the general forward rail carries a
// FlowSpec redirect-to-IP route's next hop instead of the RFC 8955 Section 4
// zero length, and resets it per destination.
//
// draft-simpson-idr-flowspec-redirect-02 Section 3: "When a flow-spec route
// with this community is advertised to an EBGP peer the next-hop address in
// the MP_REACH_NLRI SHOULD be reset to an address of the advertising router by
// default, per normal BGP procedures. Alternatively, the advertising router
// MAY be configured to keep the next-hop unchanged".
//
// Method: a route received with the community and a next hop is forwarded
// through forwardUpdateCore to one destination, and the recipient's
// MP_REACH_NLRI is compared byte for byte. IBGP under next hop auto keeps the
// received address, EBGP under auto takes the speaker's address, EBGP under
// next hop unchanged keeps it, IBGP under next hop self takes the speaker's
// address, and a destination not sent extended communities gets length 0,
// because the redirect action it would follow is not sent.
//
// MUTATION: drop the mpRedirect branch of applyNextHopFamily (every case leaves
// with length 0), or treat EBGP under auto as keep (the reset case keeps the
// received address).
func TestFlowSpecRedirectForwardNextHop(t *testing.T) {
	self4 := netip.MustParseAddr("10.0.0.254")
	for _, tc := range []struct {
		name     string
		fam      family.Family
		nlri     string
		received netip.Addr
	}{
		{"ipv4-flow", family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}, "050118c00002", netip.MustParseAddr("192.0.2.77")},
		{"ipv6-flow", family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIFlowSpec}, "0701200020010db8", netip.MustParseAddr("2001:db8::77")},
	} {
		selfHop := self4.AsSlice() // 4 octets under either AFI, never IPv4-mapped.
		for _, dest := range []struct {
			name      string
			peerAS    uint32
			mode      uint8
			community []string
			want      []byte
			action    bool
		}{
			{"ibgp-auto-keeps", 65000, NextHopAuto, nil, tc.received.AsSlice(), true},
			{"ebgp-auto-resets-to-self", 65001, NextHopAuto, nil, selfHop, true},
			{"ebgp-unchanged-keeps", 65001, NextHopUnchanged, nil, tc.received.AsSlice(), true},
			{"ibgp-self-rewrites", 65000, NextHopSelf, nil, selfHop, true},
			{"ebgp-no-extended-strips", 65001, NextHopAuto, []string{"standard"}, nil, false},
		} {
			t.Run(tc.name+"/"+dest.name, func(t *testing.T) {
				f := newAIGPReplayFixture(t, nil)
				for _, peer := range []*Peer{f.source, f.destination} {
					peer.negotiated.Store(&NegotiatedCapabilities{ASN4: true, families: map[family.Family]bool{tc.fam: true}})
				}
				f.destination.settings.PeerAS = dest.peerAS
				f.destination.settings.NextHopMode = dest.mode
				f.destination.settings.SendCommunity = dest.community
				f.destination.refreshForwardFacts()
				raw := mustHex(t, tc.nlri)
				key := ribevents.ValidationRoute{Peer: f.source.Settings().Address, Family: tc.fam, NLRI: string(raw)}
				ribevents.RegisterFlowSpecLookup(func(got ribevents.ValidationRoute, _ uint64) bool { return got == key }, nil, nil, nil)
				t.Cleanup(func() { ribevents.RegisterFlowSpecLookup(nil, nil, nil, nil) })
				id := f.receive(t, flowRedirectPayload(t, tc.fam, raw, tc.received.AsSlice()))
				f.forward(t, id)
				forwardSocketBarrier(t, f.r)
				bodies := aigpSocketBodies(t, f.conn)
				if len(bodies) != 1 {
					t.Fatalf("recipient UPDATE count=%d, want one rule", len(bodies))
				}
				assertFlowRedirectWire(t, bodies[0], tc.fam, raw, dest.want, dest.action)
			})
		}
	}
}

// flowRedirectPayload is flowForwardPayload with the redirect-to-IP action in
// place of the traffic-rate one.
func flowRedirectPayload(t *testing.T, fam family.Family, raw, hop []byte) []byte {
	t.Helper()
	attrs := mustHex(t, "4001010040020602010000fdeac01008"+flowRedirectAction)
	mp := []byte{byte(fam.AFI >> 8), byte(fam.AFI), byte(fam.SAFI), byte(len(hop))}
	mp = append(mp, hop...)
	mp = append(mp, 0)
	mp = append(mp, raw...)
	attrs = append(attrs, 0x80, 14, byte(len(mp)))
	attrs = append(attrs, mp...)
	body := binary.BigEndian.AppendUint16([]byte{0, 0}, uint16(len(attrs)))
	return append(body, attrs...)
}

// assertFlowRedirectWire checks the recipient's MP_REACH_NLRI is exactly the
// rule with hop as its next hop, and whether the redirect action reached it.
func assertFlowRedirectWire(t *testing.T, body []byte, fam family.Family, raw, hop []byte, action bool) {
	t.Helper()
	update, err := message.UnpackUpdate(body)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{byte(fam.AFI >> 8), byte(fam.AFI), byte(fam.SAFI), byte(len(hop))}
	want = append(want, hop...)
	want = append(want, 0)
	want = append(want, raw...)
	_, _, mp, found := attribute.AttrFind(update.PathAttributes, attribute.AttrMPReachNLRI)
	if !found || !bytes.Equal(mp, want) {
		t.Errorf("recipient MP_REACH=%x present=%v, want %x", mp, found, want)
	}
	_, _, actions, found := attribute.AttrFind(update.PathAttributes, attribute.AttrExtCommunity)
	if action != (found && bytes.Equal(actions, mustHex(t, flowRedirectAction))) {
		t.Errorf("recipient redirect action=%x present=%v, want sent=%v", actions, found, action)
	}
}

// TestFlowSpecRedirectRouteServerKeepsNextHop proves the route-server rail
// passes a redirect-to-IP route's next hop to an EBGP route-server client
// unchanged, where the general rail would reset it to self.
//
// RFC 7947 Section 2.2.1: "the NEXT_HOP attribute MUST be passed unmodified to
// the route server clients". The client is external (AS 65003 against the
// route server's 65000) under next hop auto, the one combination where
// draft-simpson-idr-flowspec-redirect-02 Section 3 alone would reset it.
//
// Method: the daemon's own route server, RIB authorization and TCP sessions
// (flowForwardLiveRouter) receive the covering route and the rule, and the
// client's MP_REACH_NLRI is compared byte for byte.
//
// MUTATION: drop the rsClient test from precomputeRedirectNextHop's auto case
// (the client receives the route server's connected address).
func TestFlowSpecRedirectRouteServerKeepsNextHop(t *testing.T) {
	peers := flowForwardLiveRouter(t, "auto")
	peers[0].send(t, flowForwardFrame(mustHex(t, "000000144001010040020602010000fdea400304c000020118c00002")))
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}
	raw := mustHex(t, "050118c00002")
	hop := netip.MustParseAddr("192.0.2.77").AsSlice()
	peers[0].send(t, flowForwardFrame(flowRedirectPayload(t, fam, raw, hop)))
	lowEventually(t, func() bool { return flowForwardReceived(peers[1], fam) != nil }, "authorized FlowSpec redirect announcement")
	assertFlowRedirectWire(t, flowForwardReceived(peers[1], fam), fam, raw, hop, true)
}
